// Package srp is SRP-6a (RFC 5054) as the lobby uses it: the client proves
// it knows the password without sending it, and the server proves it holds
// the verifier made from it. The server keeps only the salt and the
// verifier v = g^x, where x comes from the password through Argon2id (X).
//
// The proofs follow the common SRP-6a form: M1 = H(A | B | S),
// M2 = H(A | M1 | S), with A, B and S padded to the length of N. No session
// key is derived: the connection is TLS already.
package srp

import (
	"crypto"
	"crypto/rand"
	_ "crypto/sha256" // the hash of Group3072
	"crypto/subtle"
	"errors"
	"math/big"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Group is a prime N, a generator g and the hash H.
type Group struct {
	N, G *big.Int
	Hash crypto.Hash
}

// Group3072 is RFC 5054's 3072-bit group with SHA-256.
var Group3072 = &Group{N: hexInt(`
	FFFFFFFF FFFFFFFF C90FDAA2 2168C234 C4C6628B 80DC1CD1 29024E08
	8A67CC74 020BBEA6 3B139B22 514A0879 8E3404DD EF9519B3 CD3A431B
	302B0A6D F25F1437 4FE1356D 6D51C245 E485B576 625E7EC6 F44C42E9
	A637ED6B 0BFF5CB6 F406B7ED EE386BFB 5A899FA5 AE9F2411 7C4B1FE6
	49286651 ECE45B3D C2007CB8 A163BF05 98DA4836 1C55D39A 69163FA8
	FD24CF5F 83655D23 DCA3AD96 1C62F356 208552BB 9ED52907 7096966D
	670C354E 4ABC9804 F1746C08 CA18217C 32905E46 2E36CE3B E39E772C
	180E8603 9B2783A2 EC07A28F B5C55DF0 6F4C52C9 DE2BCBF6 95581718
	3995497C EA956AE5 15D22618 98FA0510 15728E5A 8AAAC42D AD33170D
	04507A33 A85521AB DF1CBA64 ECFB8504 58DBEF0A 8AEA7157 5D060C7D
	B3970F85 A6E1E4C7 ABF5AE8C DB0933D7 1E8C94E0 4A25619D CEE3D226
	1AD2EE6B F12FFA06 D98A0864 D8760273 3EC86A64 521F2B18 177B200C
	BBE11757 7A615D6C 770988C0 BAD946E2 08E24FA0 74E5AB31 43DB5BFC
	E0FD108E 4B82D120 A93AD2CA FFFFFFFF FFFFFFFF`), G: big.NewInt(5), Hash: crypto.SHA256}

// ErrBadValue is a peer's public value or proof that SRP must refuse.
var ErrBadValue = errors.New("srp: bad value")

// KDF is the Argon2id parameters of X. They are stored with each verifier,
// so they can grow for new accounts without breaking old ones.
type KDF struct {
	Time, MemoryKiB uint32
	Threads         uint8
}

// DefaultKDF is what new verifiers use.
var DefaultKDF = KDF{Time: 3, MemoryKiB: 64 * 1024, Threads: 1}

// SaltSize is the length of a new salt.
const SaltSize = 16

// NewSalt returns a random salt.
func NewSalt() []byte {
	s := make([]byte, SaltSize)
	rand.Read(s)
	return s
}

// X derives SRP's private x from the username (ignoring case, as names are
// unique ignoring case) and the password.
func X(username, password string, salt []byte, p KDF) *big.Int {
	key := argon2.IDKey([]byte(strings.ToLower(username)+":"+password), salt, p.Time, p.MemoryKiB, p.Threads, 32)
	return new(big.Int).SetBytes(key)
}

// Verifier is v = g^x mod N, what the server stores.
func (g *Group) Verifier(x *big.Int) []byte {
	return g.pad(new(big.Int).Exp(g.G, x, g.N))
}

func (g *Group) size() int { return (g.N.BitLen() + 7) / 8 }

// pad is n big-endian, left-padded to the length of N.
func (g *Group) pad(n *big.Int) []byte { return n.FillBytes(make([]byte, g.size())) }

func (g *Group) hash(parts ...[]byte) []byte {
	h := g.Hash.New()
	for _, p := range parts {
		h.Write(p)
	}
	return h.Sum(nil)
}

func (g *Group) hashInt(parts ...[]byte) *big.Int { return new(big.Int).SetBytes(g.hash(parts...)) }

// k is the multiplier H(N | PAD(g)).
func (g *Group) k() *big.Int { return g.hashInt(g.N.Bytes(), g.pad(g.G)) }

// value reads a peer's public value: at most the length of N and not
// 0 mod N.
func (g *Group) value(b []byte) (*big.Int, error) {
	if len(b) == 0 || len(b) > g.size() {
		return nil, ErrBadValue
	}
	n := new(big.Int).SetBytes(b)
	if new(big.Int).Mod(n, g.N).Sign() == 0 {
		return nil, ErrBadValue
	}
	return n, nil
}

// secret is a random private exponent of 256 bits.
func secret() *big.Int {
	b := make([]byte, 32)
	rand.Read(b)
	return new(big.Int).SetBytes(b)
}

// Client is the client's side of one login.
type Client struct {
	g     *Group
	a, A  *big.Int
	m1, s []byte
}

// NewClient starts a login.
func NewClient(g *Group) *Client { return newClient(g, secret()) }

func newClient(g *Group, a *big.Int) *Client {
	return &Client{g: g, a: a, A: new(big.Int).Exp(g.G, a, g.N)}
}

// Public is A, sent with the proof.
func (c *Client) Public() []byte { return c.g.pad(c.A) }

// Proof takes the server's B and the client's x and returns the proof M1.
func (c *Client) Proof(x *big.Int, serverB []byte) ([]byte, error) {
	g := c.g
	B, err := g.value(serverB)
	if err != nil {
		return nil, err
	}
	pA, pB := g.pad(c.A), g.pad(B)
	u := g.hashInt(pA, pB)
	if u.Sign() == 0 {
		return nil, ErrBadValue
	}
	// S = (B - k*g^x) ^ (a + u*x) mod N
	base := new(big.Int).Exp(g.G, x, g.N)
	base.Mul(base, g.k())
	base.Sub(B, base)
	base.Mod(base, g.N)
	exp := new(big.Int).Mul(u, x)
	exp.Add(exp, c.a)
	c.s = g.pad(new(big.Int).Exp(base, exp, g.N))
	c.m1 = g.hash(pA, pB, c.s)
	return c.m1, nil
}

// Check reports whether the server's proof M2 is right: the server holds
// the verifier of this password.
func (c *Client) Check(m2 []byte) bool {
	if c.m1 == nil {
		return false
	}
	return subtle.ConstantTimeCompare(m2, c.g.hash(c.g.pad(c.A), c.m1, c.s)) == 1
}

// Server is the server's side of one login.
type Server struct {
	g       *Group
	v, b, B *big.Int
}

// NewServer starts a login against a stored verifier.
func NewServer(g *Group, verifier []byte) *Server {
	return newServer(g, verifier, secret())
}

func newServer(g *Group, verifier []byte, b *big.Int) *Server {
	v := new(big.Int).SetBytes(verifier)
	// B = k*v + g^b mod N
	B := new(big.Int).Mul(g.k(), v)
	B.Add(B, new(big.Int).Exp(g.G, b, g.N))
	B.Mod(B, g.N)
	return &Server{g: g, v: v, b: b, B: B}
}

// Public is B, sent with the challenge.
func (s *Server) Public() []byte { return s.g.pad(s.B) }

// Check takes the client's A and proof M1 and returns the server's proof
// M2, or ErrBadValue when the proof is wrong (a wrong password).
func (s *Server) Check(clientA, m1 []byte) ([]byte, error) {
	g := s.g
	A, err := g.value(clientA)
	if err != nil {
		return nil, err
	}
	pA, pB := g.pad(A), g.pad(s.B)
	u := g.hashInt(pA, pB)
	if u.Sign() == 0 {
		return nil, ErrBadValue
	}
	// S = (A * v^u) ^ b mod N
	S := new(big.Int).Exp(s.v, u, g.N)
	S.Mul(S, A)
	S.Exp(S, s.b, g.N)
	pS := g.pad(S)
	if subtle.ConstantTimeCompare(m1, g.hash(pA, pB, pS)) != 1 {
		return nil, ErrBadValue
	}
	return g.hash(pA, m1, pS), nil
}

func hexInt(s string) *big.Int {
	n, ok := new(big.Int).SetString(strings.Join(strings.Fields(s), ""), 16)
	if !ok {
		panic("srp: bad constant")
	}
	return n
}
