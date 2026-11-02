package srp

import (
	"bytes"
	"crypto"
	_ "crypto/sha1"
	"math/big"
	"testing"
)

// The 1024-bit group and the test vectors of RFC 5054, Appendix A and B.
var group1024 = &Group{N: hexInt(`
	EEAF0AB9 ADB38DD6 9C33F80A FA8FC5E8 60726187 75FF3C0B 9EA2314C
	9C256576 D674DF74 96EA81D3 383B4813 D692C6E0 E0D5D8E2 50B98BE4
	8E495C1D 6089DAD1 5DC7D7B4 6154D6B6 CE8EF4AD 69B15D49 82559B29
	7BCF1885 C529F566 660E57EC 68EDBC3C 05726CC0 2FD4CBF4 976EAA9A
	FD5138FE 8376435B 9FC61D2F C0EB06E3`), G: big.NewInt(2), Hash: crypto.SHA1}

var vec = struct {
	I, P                   string
	s, k, x, v, a, b, A, B string
	u, S                   string
}{
	I: "alice",
	P: "password123",
	s: "BEB25379 D1A8581E B5A72767 3A2441EE",
	k: "7556AA04 5AEF2CDD 07ABAF0F 665C3E81 8913186F",
	x: "94B7555A ABE9127C C58CCF49 93DB6CF8 4D16C124",
	v: `7E273DE8 696FFC4F 4E337D05 B4B375BE B0DDE156 9E8FA00A 9886D812
		9BADA1F1 822223CA 1A605B53 0E379BA4 729FDC59 F105B478 7E5186F5
		C671085A 1447B52A 48CF1970 B4FB6F84 00BBF4CE BFBB1681 52E08AB5
		EA53D15C 1AFF87B2 B9DA6E04 E058AD51 CC72BFC9 033B564E 26480D78
		E955A5E2 9E7AB245 DB2BE315 E2099AFB`,
	a: `60975527 035CF2AD 1989806F 0407210B C81EDC04 E2762A56 AFD529DD
		DA2D4393`,
	b: `E487CB59 D31AC550 471E81F0 0F6928E0 1DDA08E9 74A004F4 9E61F5D1
		05284D20`,
	A: `61D5E490 F6F1B795 47B0704C 436F523D D0E560F0 C64115BB 72557EC4
		4352E890 3211C046 92272D8B 2D1A5358 A2CF1B6E 0BFCF99F 921530EC
		8E393561 79EAE45E 42BA92AE ACED8251 71E1E8B9 AF6D9C03 E1327F44
		BE087EF0 6530E69F 66615261 EEF54073 CA11CF58 58F0EDFD FE15EFEA
		B349EF5D 76988A36 72FAC47B 0769447B`,
	B: `BD0C6151 2C692C0C B6D041FA 01BB152D 4916A1E7 7AF46AE1 05393011
		BAF38964 DC46A067 0DD125B9 5A981652 236F99D9 B681CBF8 7837EC99
		6C6DA044 53728610 D0C6DDB5 8B318885 D7D82C7F 8DEB75CE 7BD4FBAA
		37089E6F 9C6059F3 88838E7A 00030B33 1EB76840 910440B1 B27AAEAE
		EB4012B7 D7665238 A8E3FB00 4B117B58`,
	u: "CE38B959 3487DA98 554ED47D 70A7AE5F 462EF019",
	S: `B0DC82BA BCF30674 AE450C02 87745E79 90A3381F 63B387AA F271A10D
		233861E3 59B48220 F7C4693C 9AE12B0A 6F67809F 0876E2D0 13800D6C
		41BB59B6 D5979B5C 00A172B4 A2A5903A 0BDCAF8A 709585EB 2AFAFA8F
		3499B200 210DCC1F 10EB3394 3CD67FC8 8A2F39A4 BE5BEC4E C0A3212D
		C346D7E4 74B29EDE 8A469FFE CA686E5A`,
}

func TestRFC5054Vectors(t *testing.T) {
	g := group1024
	eq := func(name string, got *big.Int, want string) {
		t.Helper()
		if got.Cmp(hexInt(want)) != 0 {
			t.Errorf("%s = %X, want %s", name, got, want)
		}
	}
	eq("k", g.k(), vec.k)
	// RFC 5054's x = H(s | H(I ":" P)); the lobby's X uses Argon2id instead.
	salt := hexInt(vec.s).Bytes()
	x := g.hashInt(salt, g.hash([]byte(vec.I+":"+vec.P)))
	eq("x", x, vec.x)
	v := g.Verifier(x)
	eq("v", new(big.Int).SetBytes(v), vec.v)

	c := newClient(g, hexInt(vec.a))
	s := newServer(g, v, hexInt(vec.b))
	eq("A", c.A, vec.A)
	eq("B", s.B, vec.B)
	eq("u", g.hashInt(c.Public(), s.Public()), vec.u)

	m1, err := c.Proof(x, s.Public())
	if err != nil {
		t.Fatal(err)
	}
	eq("S", new(big.Int).SetBytes(c.s), vec.S)
	m2, err := s.Check(c.Public(), m1)
	if err != nil {
		t.Fatalf("server refused the right proof: %v", err)
	}
	if !c.Check(m2) {
		t.Fatal("client refused the server's proof")
	}
}

func TestGroup3072(t *testing.T) {
	g := Group3072
	if g.N.BitLen() != 3072 {
		t.Fatalf("N has %d bits", g.N.BitLen())
	}
	// A safe prime: N and (N-1)/2 both prime. A mistyped constant fails this.
	q := new(big.Int).Rsh(g.N, 1)
	if !g.N.ProbablyPrime(20) || !q.ProbablyPrime(20) {
		t.Fatal("N is not a safe prime")
	}
}

func login(t *testing.T, user, stored, typed string) error {
	t.Helper()
	g := Group3072
	kdf := KDF{Time: 1, MemoryKiB: 1024, Threads: 1} // fast for tests
	salt := NewSalt()
	v := g.Verifier(X(user, stored, salt, kdf))

	s := NewServer(g, v)
	c := NewClient(g)
	m1, err := c.Proof(X(user, typed, salt, kdf), s.Public())
	if err != nil {
		return err
	}
	m2, err := s.Check(c.Public(), m1)
	if err != nil {
		return err
	}
	if !c.Check(m2) {
		t.Fatal("client refused the server's proof")
	}
	return nil
}

func TestLogin(t *testing.T) {
	if err := login(t, "Alice", "correct horse", "correct horse"); err != nil {
		t.Fatalf("right password: %v", err)
	}
	if err := login(t, "Alice", "correct horse", "correct hors"); err != ErrBadValue {
		t.Fatalf("wrong password: %v, want ErrBadValue", err)
	}
}

func TestUsernameCase(t *testing.T) {
	salt, kdf := NewSalt(), KDF{Time: 1, MemoryKiB: 1024, Threads: 1}
	if X("Alice", "pw", salt, kdf).Cmp(X("aLICE", "pw", salt, kdf)) != 0 {
		t.Fatal("x depends on the username's case")
	}
}

func TestBadValues(t *testing.T) {
	g := Group3072
	s := NewServer(g, g.Verifier(big.NewInt(12345)))
	for name, A := range map[string][]byte{
		"zero":     {0},
		"N":        g.N.Bytes(),
		"2N":       new(big.Int).Lsh(g.N, 1).Bytes(),
		"empty":    nil,
		"too long": append([]byte{1}, g.pad(big.NewInt(7))...),
	} {
		if _, err := s.Check(A, make([]byte, 32)); err != ErrBadValue {
			t.Errorf("A = %s: %v, want ErrBadValue", name, err)
		}
	}
	c := NewClient(g)
	if _, err := c.Proof(big.NewInt(1), g.N.Bytes()); err != ErrBadValue {
		t.Errorf("B = N: %v, want ErrBadValue", err)
	}
	if c.Check(bytes.Repeat([]byte{0}, 32)) {
		t.Error("Check passed before a proof")
	}
}
