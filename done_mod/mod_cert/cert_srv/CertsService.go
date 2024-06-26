package certsrv

import (
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"log/slog"
	"math/big"
	"net"
	"time"

	clidone "github.com/hiveot/hub/done_cli/cli_done"
	certapi "github.com/hiveot/hub/done_mod/mod_cert/cert_api"
	"github.com/hiveot/hub/done_tool/certs"
	"github.com/hiveot/hub/done_tool/keys"
)

// CertsService creates certificates for use by services, devices and admin users.
//
// Note that this service does not support certificate revocation.
//
// *	See also: https://www.imperialviolet.org/2014/04/19/revchecking.html
//
// Issued certificates are short-lived and must be renewed before they expire.
type CertsService struct {
	caCert     *x509.Certificate
	caCertPEM  string
	caKey      keys.IHiveKey
	caCertPool *x509.CertPool

	// messaging client for receiving requests
	hc *clidone.HubClient
}

// _createDeviceCert internal function to create a CA signed certificate for mutual authentication by IoT devices
func (svc *CertsService) _createDeviceCert(
	deviceID string, pubKey keys.IHiveKey, validityDays int) (
	cert *x509.Certificate, err error) {
	if validityDays == 0 {
		validityDays = certapi.DefaultDeviceCertValidityDays
	}

	cert, err = createClientCert(
		deviceID,
		certs.OUIoTDevice,
		pubKey,
		svc.caCert,
		svc.caKey,
		validityDays)

	// TODO: send Thing event (services are things too)
	return cert, err
}

// createServiceCert internal function to create a CA signed service certificate for mutual authentication between services
func (svc *CertsService) _createServiceCert(
	serviceID string, pubKey keys.IHiveKey, names []string, validityDays int) (
	cert *x509.Certificate, err error) {

	if serviceID == "" || pubKey == nil || names == nil {
		err := fmt.Errorf("missing argument serviceID, servicePubKey, or names")
		slog.Error(err.Error())
		return nil, err
	}
	if validityDays == 0 {
		validityDays = certapi.DefaultServiceCertValidityDays
	}

	// firefox complains if serial is the same as that of the CA. So generate a unique one based on timestamp.
	serial := time.Now().Unix() - 3
	template := &x509.Certificate{
		SerialNumber: big.NewInt(serial),
		Subject: pkix.Name{
			Country:            []string{"CA"},
			Province:           []string{"BC"},
			Locality:           []string{"local"},
			Organization:       []string{"HiveOT"},
			OrganizationalUnit: []string{certs.OUService},
			CommonName:         serviceID,
		},
		NotBefore: time.Now().Add(-time.Second),
		NotAfter:  time.Now().AddDate(0, 0, validityDays),
		//NotBefore: time.Now(),
		//NotAfter:  time.Now().AddDate(0, 0, config.DefaultServiceCertDurationDays),

		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageDataEncipherment | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		//ExtKeyUsage:    []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IsCA:           false,
		MaxPathLenZero: true,
		// BasicConstraintsValid: true,
		// IPAddresses: []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
		IPAddresses: []net.IP{},
	}
	// determine the hosts for this hub
	for _, h := range names {
		if ip := net.ParseIP(h); ip != nil {
			template.IPAddresses = append(template.IPAddresses, ip)
		} else {
			template.DNSNames = append(template.DNSNames, h)
		}
	}
	// Create the service private key
	//certKey := certs.CreateECDSAKeys()
	// and the certificate itself
	certDer, err := x509.CreateCertificate(rand.Reader, template,
		svc.caCert, pubKey.PublicKey(), svc.caKey.PrivateKey())
	if err == nil {
		cert, err = x509.ParseCertificate(certDer)
	}

	// TODO: send Thing event (services are things too)
	return cert, err
}

// _createUserCert internal function to create a client certificate for end-users
func (svc *CertsService) _createUserCert(userID string, pubKey keys.IHiveKey, validityDays int) (
	cert *x509.Certificate, err error) {
	if validityDays == 0 {
		validityDays = certapi.DefaultUserCertValidityDays
	}

	cert, err = createClientCert(
		userID,
		certs.OUUser,
		pubKey,
		svc.caCert,
		svc.caKey,
		validityDays)
	// TODO: send Thing event (services are things too)
	return cert, err
}

// CreateDeviceCert creates a CA signed certificate for mutual authentication by IoT devices in PEM format
func (svc *CertsService) CreateDeviceCert(
	ctx clidone.ServiceContext, args certapi.CreateDeviceCertArgs) (certapi.CreateCertResp, error) {
	//deviceID string, pubKeyPEM string, durationDays int) (
	//certPEM string, caCertPEM string, err error) {
	var cert *x509.Certificate

	slog.Info("CreateDeviceCert", "deviceID", args.DeviceID, "pubKey", args.PubKeyPEM)
	k := keys.NewKey(keys.KeyTypeECDSA)
	err := k.ImportPublic(args.PubKeyPEM)
	if err != nil {
		err = fmt.Errorf("public key for '%s' is invalid: %s", args.DeviceID, err)
	} else {
		cert, err = svc._createDeviceCert(args.DeviceID, k, args.ValidityDays)
	}
	resp := certapi.CreateCertResp{}
	if err == nil {
		resp.CertPEM = certs.X509CertToPEM(cert)
		resp.CaCertPEM = svc.caCertPEM
	}
	return resp, err
}

// CreateServiceCert creates a CA signed service certificate for mutual authentication between services
func (svc *CertsService) CreateServiceCert(
	ctx clidone.ServiceContext, args certapi.CreateServiceCertArgs) (certapi.CreateCertResp, error) {
	var cert *x509.Certificate

	slog.Info("Creating service certificate",
		"serviceID", args.ServiceID, "names", args.Names)
	k := keys.NewKey(keys.KeyTypeECDSA)
	err := k.ImportPublic(args.PubKeyPEM)
	if err == nil {
		cert, err = svc._createServiceCert(args.ServiceID, k, args.Names, args.ValidityDays)
	}
	resp := certapi.CreateCertResp{}
	if err == nil {
		resp.CertPEM = certs.X509CertToPEM(cert)
		resp.CaCertPEM = svc.caCertPEM
	}
	// TODO: send event. is there a use-case for limiting service events to roles?
	return resp, err
}

// CreateUserCert creates a client certificate for end-users
func (svc *CertsService) CreateUserCert(
	ctx clidone.ServiceContext, args certapi.CreateUserCertArgs) (certapi.CreateCertResp, error) {
	//userID string, pubKeyPEM string, validityDays int) (
	//certPEM string, caCertPEM string, err error) {
	var cert *x509.Certificate

	slog.Info("CreateUserCert",
		"userID", args.UserID, "pubKey", args.PubKeyPEM)
	k := keys.NewKey(keys.KeyTypeECDSA)
	err := k.ImportPublic(args.PubKeyPEM)
	if err == nil {

		cert, err = svc._createUserCert(args.UserID, k, args.ValidityDays)
	}
	resp := certapi.CreateCertResp{}
	if err == nil {
		resp.CertPEM = certs.X509CertToPEM(cert)
		resp.CaCertPEM = svc.caCertPEM
	}
	return resp, err
}

// Start the service and listen for requests
//
//	hc is the connection to the hub with a service role. For testing it can be nil.
func (svc *CertsService) Start(hc *clidone.HubClient) (err error) {
	slog.Warn("Starting certs service", "serviceID", hc.ClientID())
	// for testing, hc can be nil
	svc.hc = hc
	svc.hc.SetRPCCapability(certapi.ManageCertsCapability,
		map[string]interface{}{
			certapi.CreateDeviceCertMethod:  svc.CreateDeviceCert,
			certapi.CreateServiceCertMethod: svc.CreateServiceCert,
			certapi.CreateUserCertMethod:    svc.CreateUserCert,
			certapi.VerifyCertMethod:        svc.VerifyCert,
		})

	return err
}

// Stop the service and remove subscription
func (svc *CertsService) Stop() {
	slog.Warn("Stopping the certs service")
}

// VerifyCert verifies whether the given certificate is a valid client certificate
func (svc *CertsService) VerifyCert(ctx clidone.ServiceContext, args certapi.VerifyCertArgs) error {

	opts := x509.VerifyOptions{
		Roots:     svc.caCertPool,
		KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	cert, err := certs.X509CertFromPEM(args.CertPEM)
	if err == nil {
		if cert.Subject.CommonName != args.ClientID {
			err = fmt.Errorf("client ID '%s' doesn't match certificate name '%s'",
				args.ClientID, cert.Subject.CommonName)
		}
	}
	//if err == nil {
	//	x509Cert, err := x509.ParseCertificate(clientCert.Certificate[0])
	//}
	if err == nil {
		_, err = cert.Verify(opts)
	}
	return err
}

// NewCertsService returns a new instance of the selfsigned certificate service
//
//	caCert is the CA certificate used to created certificates
//	caKey is the CA private key used to created certificates
func NewCertsService(
	caCert *x509.Certificate,
	caKey keys.IHiveKey,
) *CertsService {

	caCertPool := x509.NewCertPool()
	caCertPool.AddCert(caCert)

	// Use one service instance per capability.
	// This does open the door to creating an instance per client session with embedded constraints,
	// although this is not needed at the moment.
	service := &CertsService{
		caCert:     caCert,
		caKey:      caKey,
		caCertPEM:  certs.X509CertToPEM(caCert),
		caCertPool: caCertPool,
	}
	if caCert == nil || caKey == nil || caCert.PublicKey == nil {
		panic("Missing CA certificate or key")
	}

	return service
}
