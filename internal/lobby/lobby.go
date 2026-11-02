// Package lobby is the game's side of the online lobby (docs/lobby/client.md):
// the settings in lobby.yaml, the remembered session in lobby.session, and
// the connection with its login. The protocol and its client are in
// github.com/openorion2/openorion2/lobbynet; the server is a separate repository that imports them.
package lobby

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// DefaultServer is the lobby when lobby.yaml does not name one.
const DefaultServer = "galacticcouncil.openorion2.org:52530"

// Settings is lobby.yaml in the game's directory.
type Settings struct {
	Server string      `yaml:"server"` // host:port of the lobby's game port
	TLS    *tls.Config `yaml:"-"`      // nil: the system's roots (tests trust their own)
	// CA, port-only and for development: a PEM file of the certificates
	// the lobby's must be signed by instead of the system's roots
	// (tools/lobbyfake).
	CA string `yaml:"ca,omitempty"`
}

const (
	settingsFile = "lobby.yaml"
	sessionFile  = "lobby.session"
)

// LoadSettings reads dir/lobby.yaml, writing it with the defaults when it
// is missing so that players find it to edit.
func LoadSettings(dir string) (Settings, error) {
	s := Settings{Server: DefaultServer}
	path := filepath.Join(dir, settingsFile)
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		out, _ := yaml.Marshal(s)
		os.WriteFile(path, append([]byte("# The online lobby (Multiplayer, Total Entertainment Network).\n"), out...), 0o644)
		return s, nil
	}
	if err != nil {
		return s, err
	}
	if err := yaml.Unmarshal(b, &s); err != nil {
		return s, err
	}
	if s.Server == "" {
		s.Server = DefaultServer
	}
	if s.CA != "" {
		ca := s.CA
		if !filepath.IsAbs(ca) {
			ca = filepath.Join(dir, ca)
		}
		pemBytes, err := os.ReadFile(ca)
		if err != nil {
			return s, err
		}
		pool := x509.NewCertPool() // that lobby only: on macOS the system pool ignores additions
		if !pool.AppendCertsFromPEM(pemBytes) {
			return s, fmt.Errorf("lobby.yaml: no certificate in %s", ca)
		}
		s.TLS = &tls.Config{RootCAs: pool}
	}
	return s, nil
}

// Remembered is lobby.session: the last username, and the session that
// logs in without the password until it expires or the player logs out.
type Remembered struct {
	User    string `json:"user"`
	Session string `json:"session,omitempty"`
}

// LoadRemembered reads dir/lobby.session; a missing or broken file is an
// empty one.
func LoadRemembered(dir string) Remembered {
	var r Remembered
	if b, err := os.ReadFile(filepath.Join(dir, sessionFile)); err == nil {
		json.Unmarshal(b, &r)
	}
	return r
}

// Save writes dir/lobby.session, readable by the player only: the session
// logs in as him.
func (r Remembered) Save(dir string) error {
	b, err := json.Marshal(r)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, sessionFile), b, 0o600)
}

// linkFile is the file next to SAVE<slot+1>.GAM naming the lobby game the
// save was made in.
func linkFile(dir string, slot int) string {
	return filepath.Join(dir, fmt.Sprintf("SAVE%d.LBY", slot+1))
}

// SaveLink records that save slot was made in lobby game id; an empty id
// (a save outside the lobby) removes the record.
func SaveLink(dir string, slot int, id string) error {
	if id == "" {
		err := os.Remove(linkFile(dir, slot))
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	return os.WriteFile(linkFile(dir, slot), []byte(id+"\n"), 0o644)
}

// LoadLink is the lobby game save slot was made in, "" if none.
func LoadLink(dir string, slot int) string {
	b, err := os.ReadFile(linkFile(dir, slot))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}
