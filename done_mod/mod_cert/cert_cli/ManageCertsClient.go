package certcli

import (
	clidone "github.com/hiveot/hub/done_cli/cli_done"
	certapi "github.com/hiveot/hub/done_mod/mod_cert/cert_api"
)

// CertsClient is a marshaller for cert service messages using a provided hub connection.
// This uses the default serializer to marshal and unmarshal messages.
type CertsClient struct {
	agentID string
	capID   string
	hc      *clidone.HubClient
}

// CreateDeviceCert generates or renews IoT device certificate for access hub IoT gateway
func (cl *CertsClient) CreateDeviceCert(deviceID string, pubKeyPEM string, validityDays int) (
	certPEM string, caCertPEM string, err error) {

	req := certapi.CreateDeviceCertArgs{
		DeviceID:     deviceID,
		PubKeyPEM:    pubKeyPEM,
		ValidityDays: validityDays,
	}
	resp := certapi.CreateCertResp{}
	err = cl.hc.PubRPCRequest(
		cl.agentID, cl.capID, certapi.CreateDeviceCertMethod, req, &resp)
	return resp.CertPEM, resp.CaCertPEM, err
}

// CreateServiceCert generates or renews service certificates for access hub IoT gateway
func (cl *CertsClient) CreateServiceCert(
	serviceID string, pubKeyPEM string, names []string, validityDays int) (
	certPEM string, caCertPEM string, err error) {

	req := certapi.CreateServiceCertArgs{
		ServiceID:    serviceID,
		PubKeyPEM:    pubKeyPEM,
		Names:        names,
		ValidityDays: validityDays,
	}
	resp := certapi.CreateCertResp{}
	err = cl.hc.PubRPCRequest(
		cl.agentID, cl.capID, certapi.CreateServiceCertMethod, req, &resp)

	return resp.CertPEM, resp.CaCertPEM, err
}

// CreateUserCert generates or renews user certificates for access hiveot hub
func (cl *CertsClient) CreateUserCert(
	userID string, pubKeyPEM string, validityDays int) (
	certPEM string, caCertPEM string, err error) {

	req := certapi.CreateUserCertArgs{
		UserID:       userID,
		PubKeyPEM:    pubKeyPEM,
		ValidityDays: validityDays,
	}
	resp := certapi.CreateCertResp{}
	err = cl.hc.PubRPCRequest(
		cl.agentID, cl.capID, certapi.CreateUserCertMethod, req, &resp)
	return resp.CertPEM, resp.CaCertPEM, err
}

// VerifyCert verifies if the certificate is valid for the Hub
func (cl *CertsClient) VerifyCert(
	clientID string, certPEM string) (err error) {

	req := certapi.VerifyCertArgs{
		ClientID: clientID,
		CertPEM:  certPEM,
	}
	err = cl.hc.PubRPCRequest(
		cl.agentID, cl.capID, certapi.VerifyCertMethod, req, nil)
	return err
}

// NewCertsClient returns a certs service client for managing certificates
//
//	hc is the hub client connection to use
func NewCertsClient(hc *clidone.HubClient) *CertsClient {
	cl := CertsClient{
		hc:      hc,
		agentID: certapi.ServiceName,
		capID:   certapi.ManageCertsCapability,
	}
	return &cl
}
