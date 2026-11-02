// Package lobbynet holds the game's side of the online lobby's protocol,
// shared with the lobby server (a separate project that imports these
// packages):
//
//   - proto/lobby/v1/lobby.proto and lobbyv1, its generated code
//     (go generate ./lobbynet/lobbyv1) and the protocol version;
//   - wire: the framing of envelopes on a connection;
//   - srp: SRP-6a with Argon2id, the password login;
//   - client: the connection, its requests and pushes.
//
// They are outside internal/ so that the server can import them; nothing
// here depends on the rest of the game. The game's side: docs/lobby/client.md.
package lobbynet
