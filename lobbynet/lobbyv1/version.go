// Package lobbyv1 is the generated code of ../proto/lobby/v1/lobby.proto,
// plus the protocol version.
package lobbyv1

//go:generate protoc -I ../proto --go_out=. --go_opt=module=github.com/openorion2/openorion2/lobbynet/lobbyv1 lobby/v1/lobby.proto

// Protocol is the version of the lobby protocol. Bump it on any change a
// peer of the previous version would misread; the server refuses clients
// of another version (VersionMismatch).
const Protocol = 1
