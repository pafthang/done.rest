package authstr

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path"
	"sync"
	"time"

	authapi "github.com/hiveot/hub/done_mod/mod_auth/auth_api"
	modbus "github.com/hiveot/hub/done_mod/mod_bus"

	"github.com/fsnotify/fsnotify"
	"golang.org/x/crypto/bcrypt"

	"github.com/hiveot/hub/done_tool/watcher"
)

// AuthnFileStore stores client data, including users, devices and services.
// User passwords are stored using ARGON2id hash
// It includes a file watcher to automatically reload on update.
type AuthnFileStore struct {
	entries   map[string]authapi.AuthnEntry // map [loginID]"loginID:hash:userName:updated:
	storePath string
	watcher   *fsnotify.Watcher
	mutex     sync.RWMutex
}

// Add a new client.
// clientID, clientType are required, the rest is optional
func (authnStore *AuthnFileStore) Add(clientID string, profile authapi.ClientProfile) error {

	authnStore.mutex.Lock()
	defer authnStore.mutex.Unlock()

	entry, found := authnStore.entries[clientID]
	if clientID == "" || clientID != profile.ClientID {
		return fmt.Errorf("clientID or clientType are missing")
	} else if profile.ClientType != authapi.ClientTypeDevice &&
		profile.ClientType != authapi.ClientTypeUser &&
		profile.ClientType != authapi.ClientTypeService {
		return fmt.Errorf("invalid clientType '%s'", profile.ClientType)
	}
	if profile.TokenValidityDays == 0 {
		if profile.ClientType == authapi.ClientTypeDevice {
			profile.TokenValidityDays = authapi.DefaultDeviceTokenValidityDays
		} else if profile.ClientType == authapi.ClientTypeService {
			profile.TokenValidityDays = authapi.DefaultServiceTokenValidityDays
		} else if profile.ClientType == authapi.ClientTypeUser {
			profile.TokenValidityDays = authapi.DefaultUserTokenValidityDays
		}
	}
	if !found {
		slog.Debug("Adding client " + clientID)
		entry = authapi.AuthnEntry{ClientProfile: profile}
	} else {
		slog.Debug("Updating client " + clientID)
		entry.ClientProfile = profile
	}
	entry.UpdatedMSE = time.Now().UnixMilli()

	authnStore.entries[clientID] = entry

	err := authnStore.save()
	return err
}

// Close the store
func (authnStore *AuthnFileStore) Close() {
	authnStore.mutex.Lock()
	defer authnStore.mutex.Unlock()
	if authnStore.watcher != nil {
		_ = authnStore.watcher.Close()
		authnStore.watcher = nil
	}
}

// Count nr of entries in the store
func (authnStore *AuthnFileStore) Count() int {
	authnStore.mutex.RLock()
	defer authnStore.mutex.RUnlock()

	return len(authnStore.entries)
}

// GetAuthClientList provides a list of clients to apply to the message server
func (authnStore *AuthnFileStore) GetAuthClientList() []modbus.ClientAuthInfo {
	entries := authnStore.GetEntries()
	clients := make([]modbus.ClientAuthInfo, 0, len(entries))
	for _, e := range entries {
		clients = append(clients, modbus.ClientAuthInfo{
			ClientID:     e.ClientID,
			ClientType:   e.ClientType,
			PubKey:       e.PubKey,
			PasswordHash: e.PasswordHash,
			Role:         e.Role,
		})
	}
	return clients
}

// GetProfile returns the client's profile
func (authnStore *AuthnFileStore) GetProfile(clientID string) (profile authapi.ClientProfile, err error) {
	authnStore.mutex.RLock()
	defer authnStore.mutex.RUnlock()
	// user must exist
	entry, found := authnStore.entries[clientID]
	if !found {
		err = fmt.Errorf("clientID '%s' does not exist", clientID)
	}
	return entry.ClientProfile, err
}

// GetProfiles returns a list of all client profiles in the store
func (authnStore *AuthnFileStore) GetProfiles() (profiles []authapi.ClientProfile, err error) {
	authnStore.mutex.RLock()
	defer authnStore.mutex.RUnlock()
	profiles = make([]authapi.ClientProfile, 0, len(authnStore.entries))
	for _, entry := range authnStore.entries {
		profiles = append(profiles, entry.ClientProfile)
	}
	return profiles, nil
}

// GetEntries returns a list of all profiles with their hashed passwords
func (authnStore *AuthnFileStore) GetEntries() (entries []authapi.AuthnEntry) {
	authnStore.mutex.RLock()
	defer authnStore.mutex.RUnlock()
	entries = make([]authapi.AuthnEntry, 0, len(authnStore.entries))
	for _, entry := range authnStore.entries {
		entries = append(entries, entry)
	}
	return entries
}

// Open the store
// This reads the password file and subscribes to file changes
func (authnStore *AuthnFileStore) Open() (err error) {
	if authnStore.watcher != nil {
		err = fmt.Errorf("password file store '%s' is already open", authnStore.storePath)
	}
	if err == nil {
		err = authnStore.Reload()
	}
	if err == nil {
		authnStore.watcher, err = watcher.WatchFile(authnStore.storePath, authnStore.Reload)
	}
	if err != nil {
		err = fmt.Errorf("NewSession failed %w", err)
	}
	return err
}

// Reload the password store from file and subscribe to file changes
//
// If the file does not exist, it will be created.
// Returns an error if the file could not be opened/created.
func (authnStore *AuthnFileStore) Reload() error {
	authnStore.mutex.Lock()
	defer authnStore.mutex.Unlock()

	entries := make(map[string]authapi.AuthnEntry)
	dataBytes, err := os.ReadFile(authnStore.storePath)
	if errors.Is(err, os.ErrNotExist) {
		err = authnStore.save()
	} else if err != nil {
		err = fmt.Errorf("error reading password file: %w", err)
		return err
	} else if len(dataBytes) == 0 {
		// nothing to do
	} else {

		err = json.Unmarshal(dataBytes, &entries)
		if err != nil {
			err := fmt.Errorf("error while parsing password file: %w", err)
			return err
		}
		authnStore.entries = entries
	}
	return err
}

// Remove a client from the store
func (authnStore *AuthnFileStore) Remove(clientID string) (err error) {
	authnStore.mutex.Lock()
	defer authnStore.mutex.Unlock()

	_, found := authnStore.entries[clientID]
	if found {
		delete(authnStore.entries, clientID)
	}
	err = authnStore.save()
	return err
}

// save the password data to file
// if the storage folder doesn't exist it will be created
// not concurrent save
func (authnStore *AuthnFileStore) save() error {

	folder := path.Dir(authnStore.storePath)
	// ensure the location exists
	err := os.MkdirAll(folder, 0700)
	if err != nil {
		return err
	}
	tmpPath, err := WritePasswordsToTempFile(folder, authnStore.entries)
	if err != nil {
		err = fmt.Errorf("writing password file to temp failed: %w", err)
		return err
	}

	err = os.Rename(tmpPath, authnStore.storePath)
	if err != nil {
		err = fmt.Errorf("rename to password file failed: %w", err)
		return err
	}
	return err
}

// SetPassword generates and stores the user's password hash
// bcrypt limits max password length to 72 bytes
func (authnStore *AuthnFileStore) SetPassword(loginID string, password string) (err error) {
	var hash string
	if len(password) < 5 {
		return fmt.Errorf("password too short (%d chars)", len(password))
	}

	hashBytes, err2 := bcrypt.GenerateFromPassword([]byte(password), 0)
	err = err2
	hash = string(hashBytes)

	if err != nil {
		return err
	}
	return authnStore.SetPasswordHash(loginID, hash)
}

// SetPasswordHash adds/updates the password hash for the given login ID
// Intended for use by administrators to add a new user or clients to update their password
func (authnStore *AuthnFileStore) SetPasswordHash(loginID string, hash string) (err error) {
	authnStore.mutex.Lock()
	defer authnStore.mutex.Unlock()

	entry, found := authnStore.entries[loginID]
	if !found {
		return fmt.Errorf("client '%s' not found", loginID)
	}
	entry.PasswordHash = hash
	entry.UpdatedMSE = time.Now().UnixMilli()
	authnStore.entries[loginID] = entry

	err = authnStore.save()
	return err
}

// Update updates the client profile, except
func (authnStore *AuthnFileStore) Update(clientID string, profile authapi.ClientProfile) error {
	authnStore.mutex.Lock()
	defer authnStore.mutex.Unlock()

	entry, found := authnStore.entries[clientID]
	if !found {
		return fmt.Errorf("client '%s' not found", clientID)
	}
	if profile.ClientID != clientID {
		return fmt.Errorf("clientID '%s' mismatch in profile as '%s'", clientID, profile.ClientID)
	}
	if profile.ClientType != "" {
		entry.ClientType = profile.ClientType
	}
	if profile.DisplayName != "" {
		entry.DisplayName = profile.DisplayName
	}
	if profile.TokenValidityDays != 0 {
		entry.TokenValidityDays = profile.TokenValidityDays
	}
	if profile.PubKey != "" {
		entry.PubKey = profile.PubKey
	}
	entry.UpdatedMSE = time.Now().UnixMilli()
	authnStore.entries[clientID] = entry

	err := authnStore.save()
	return err
}

// VerifyPassword verifies the given password with the stored hash
// This returns the matching user's entry or an error if the password doesn't match
func (authnStore *AuthnFileStore) VerifyPassword(loginID, password string) (profile authapi.ClientProfile, err error) {
	isValid := false
	authnStore.mutex.Lock()
	defer authnStore.mutex.Unlock()

	entry, found := authnStore.entries[loginID]
	if !found {
		// unknown user
		isValid = false
	} else {
		err := bcrypt.CompareHashAndPassword([]byte(entry.PasswordHash), []byte(password))
		isValid = err == nil
	}
	if !isValid {
		return profile, fmt.Errorf("invalid login as '%s'", loginID)
	}
	profile = entry.ClientProfile
	return profile, nil
}

// WritePasswordsToTempFile write the given entries to temp file in the given folder
// This returns the name of the new temp file.
func WritePasswordsToTempFile(
	folder string, entries map[string]authapi.AuthnEntry) (tempFileName string, err error) {

	file, err := os.CreateTemp(folder, "hub-pwfilestore")

	// file, err := os.OpenFile(path, os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		err := fmt.Errorf("failed open temp password file: %s", err)
		return "", err
	}
	tempFileName = file.Name()

	defer file.Close()
	pwData, err := json.Marshal(entries)
	if err == nil {
		_, err = file.Write(pwData)
	}

	return tempFileName, err
}

// NewAuthnFileStore creates a new instance of a file based identity store.
// Call NewSession/Release to start/stop using this store.
// Note: this store is intended for one writer and many readers.
// Multiple concurrent writes are not supported and might lead to one write being ignored.
//
//	filepath location of the file store. See also DefaultPasswordFile for the recommended name
//	hashAlgo PWHASH_ARGON2id (default) or PWHASH_BCRYPT
func NewAuthnFileStore(filepath string) *AuthnFileStore {

	store := &AuthnFileStore{
		storePath: filepath,
		entries:   make(map[string]authapi.AuthnEntry),
	}
	return store
}
