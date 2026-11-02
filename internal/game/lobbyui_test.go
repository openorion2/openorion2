package game

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/openorion2/openorion2/internal/engine"
	"github.com/openorion2/openorion2/internal/host"
	"github.com/openorion2/openorion2/internal/lobby"

	"github.com/openorion2/openorion2/lobbynet/lobbyv1"
	"github.com/openorion2/openorion2/lobbynet/srp"
	"github.com/openorion2/openorion2/lobbynet/wire"
)

// fakeLobby is a lobby server with one account, alice / "correct horse",
// enough for the login: SRP, Resume, Logout. The real server's tests are
// in the lobby server's repository.
type fakeLobby struct {
	addr string
	tls  *tls.Config // the game's side: trusts the fake's certificate

	mu       sync.Mutex
	sessions map[string]bool
	logins   []string               // what happened, in order
	more     int                    // games listed besides bob's
	online   int                    // users online besides Alice and bob
	private  []*lobbyv1.PrivateLine // in the snapshot
}

const (
	fakeSession   = "11111111-2222-4333-8444-555555555555"
	fakeGameID    = "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee"
	fakeNewGameID = "12345678-1234-4234-8234-123456789abc"
)

func fakeGame(description, creator, id string) *lobbyv1.GameInfo {
	return &lobbyv1.GameInfo{Id: id, Description: description, Creator: creator, Public: true,
		State: lobbyv1.GameState_GAME_STATE_OPEN, Players: []*lobbyv1.Player{{Name: creator, NetId: 1}}}
}

func (f *fakeLobby) users() []*lobbyv1.User {
	f.mu.Lock()
	defer f.mu.Unlock()
	us := []*lobbyv1.User{{Name: "Alice"}, {Name: "bob"}}
	for i := range f.online {
		us = append(us, &lobbyv1.User{Name: fmt.Sprintf("user%02d", i)})
	}
	return us
}

func (f *fakeLobby) games() []*lobbyv1.GameInfo {
	f.mu.Lock()
	defer f.mu.Unlock()
	gs := []*lobbyv1.GameInfo{fakeGame("bob's game: Галактика 大銀河", "bob", fakeGameID)}
	for i := range f.more {
		gs = append(gs, fakeGame(fmt.Sprintf("game %d", i), "carol", fmt.Sprintf("00000000-0000-4000-8000-%012d", i)))
	}
	return gs
}

var fakeKDF = srp.KDF{Time: 1, MemoryKiB: 1024, Threads: 1}

func startFakeLobby(t *testing.T) *fakeLobby {
	t.Helper()
	srv, cli := testTLS(t)
	ln, err := tls.Listen("tcp", "127.0.0.1:0", srv)
	if err != nil {
		t.Skip(err)
	}
	t.Cleanup(func() { ln.Close() })
	f := &fakeLobby{addr: ln.Addr().String(), tls: cli, sessions: map[string]bool{}}
	salt := []byte("0123456789abcdef")
	verifier := srp.Group3072.Verifier(srp.X("alice", "correct horse", salt, fakeKDF))
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go f.serve(wire.NewConn(c), salt, verifier)
		}
	}()
	return f
}

func (f *fakeLobby) note(s string) {
	f.mu.Lock()
	f.logins = append(f.logins, s)
	f.mu.Unlock()
}

func (f *fakeLobby) serve(w *wire.Conn, salt, verifier []byte) {
	defer w.Close()
	reply := func(env *lobbyv1.Envelope, out *lobbyv1.Envelope) { out.Id = env.Id; w.Send(out) }
	failed := func(env *lobbyv1.Envelope, r lobbyv1.LoginFailure) {
		reply(env, &lobbyv1.Envelope{Msg: &lobbyv1.Envelope_LoginFailed{LoginFailed: &lobbyv1.LoginFailed{Reason: r}}})
	}
	ok := func(env *lobbyv1.Envelope, m2 []byte) {
		reply(env, &lobbyv1.Envelope{Msg: &lobbyv1.Envelope_LoginOk{LoginOk: &lobbyv1.LoginOk{
			M2: m2, Session: fakeSession, User: &lobbyv1.User{Name: "Alice"}}}})
		w.Send(&lobbyv1.Envelope{Msg: &lobbyv1.Envelope_Snapshot{Snapshot: &lobbyv1.LobbySnapshot{
			Online: f.users(),
			Chat:   []*lobbyv1.ChatLine{{User: "bob", Text: "anyone up for a huge galaxy? ¿Jugamos? Привет! 你好 こんにちは 안녕하세요"}},
			Games:  f.games(),
			Private: func() []*lobbyv1.PrivateLine {
				f.mu.Lock()
				defer f.mu.Unlock()
				return f.private
			}(),
		}}})
	}
	game := func(env *lobbyv1.Envelope, g *lobbyv1.GameInfo, id uint32) {
		reply(env, &lobbyv1.Envelope{Msg: &lobbyv1.Envelope_GameJoined{GameJoined: &lobbyv1.GameJoined{
			Game: g, NetId: id, Open: true}}})
	}
	var s *srp.Server
	var known bool
	lines := uint64(100) // the private lines' ids
	for {
		env, err := w.Recv()
		if err != nil {
			return
		}
		switch m := env.Msg.(type) {
		case *lobbyv1.Envelope_Hello:
			w.Send(&lobbyv1.Envelope{Msg: &lobbyv1.Envelope_Welcome{Welcome: &lobbyv1.Welcome{Protocol: lobbyv1.Protocol}}})
		case *lobbyv1.Envelope_LoginStart:
			known = m.LoginStart.Username == "alice"
			s = srp.NewServer(srp.Group3072, verifier)
			reply(env, &lobbyv1.Envelope{Msg: &lobbyv1.Envelope_LoginChallenge{LoginChallenge: &lobbyv1.LoginChallenge{
				Salt: salt, B: s.Public(),
				Kdf: &lobbyv1.Kdf{Time: fakeKDF.Time, MemoryKib: fakeKDF.MemoryKiB, Threads: uint32(fakeKDF.Threads)}}}})
		case *lobbyv1.Envelope_LoginProof:
			m2, err := s.Check(m.LoginProof.A, m.LoginProof.M1)
			if err != nil || !known {
				f.note("wrong")
				failed(env, lobbyv1.LoginFailure_LOGIN_FAILURE_WRONG)
				continue
			}
			f.mu.Lock()
			f.sessions[fakeSession] = true
			f.mu.Unlock()
			f.note("login")
			ok(env, m2)
		case *lobbyv1.Envelope_Resume:
			f.mu.Lock()
			live := f.sessions[m.Resume.Session]
			f.mu.Unlock()
			if !live {
				f.note("resume gone")
				failed(env, lobbyv1.LoginFailure_LOGIN_FAILURE_SESSION_GONE)
				continue
			}
			f.note("resume")
			ok(env, nil)
		case *lobbyv1.Envelope_Say:
			f.note("say " + m.Say.Text)
			reply(env, &lobbyv1.Envelope{Msg: &lobbyv1.Envelope_Ok{Ok: &lobbyv1.Ok{}}})
			w.Send(&lobbyv1.Envelope{Msg: &lobbyv1.Envelope_Chat{Chat: &lobbyv1.ChatLine{User: "Alice", Text: m.Say.Text}}})
		case *lobbyv1.Envelope_Whisper:
			// bob and carol exist; bob answers at once.
			to := strings.ToLower(m.Whisper.To)
			f.note("whisper " + to + " " + m.Whisper.Text)
			if to != "bob" && to != "carol" {
				reply(env, &lobbyv1.Envelope{Msg: &lobbyv1.Envelope_Error{Error: &lobbyv1.Error{
					Code: lobbyv1.ErrorCode_ERROR_CODE_NO_SUCH_USER, Text: "There is no such user."}}})
				continue
			}
			reply(env, &lobbyv1.Envelope{Msg: &lobbyv1.Envelope_Ok{Ok: &lobbyv1.Ok{}}})
			lines++
			w.Send(&lobbyv1.Envelope{Msg: &lobbyv1.Envelope_Private{Private: &lobbyv1.PrivateLine{
				Id: lines, From: "Alice", To: to, Text: m.Whisper.Text}}})
			if to == "bob" {
				lines++
				w.Send(&lobbyv1.Envelope{Msg: &lobbyv1.Envelope_Private{Private: &lobbyv1.PrivateLine{
					Id: lines, From: "bob", To: "Alice", Text: "re: " + m.Whisper.Text}}})
			}
		case *lobbyv1.Envelope_ReadPrivate:
			f.note(fmt.Sprintf("read %s %d", m.ReadPrivate.From, m.ReadPrivate.UpTo))
			reply(env, &lobbyv1.Envelope{Msg: &lobbyv1.Envelope_Ok{Ok: &lobbyv1.Ok{}}})
		case *lobbyv1.Envelope_CreateGame:
			f.note(fmt.Sprintf("create %s %v", m.CreateGame.Description, m.CreateGame.Public))
			game(env, fakeGame(m.CreateGame.Description, "Alice", fakeNewGameID), 1)
		case *lobbyv1.Envelope_JoinGame:
			f.note("join " + m.JoinGame.Game)
			if m.JoinGame.Game != fakeGameID {
				reply(env, &lobbyv1.Envelope{Msg: &lobbyv1.Envelope_Error{Error: &lobbyv1.Error{
					Code: lobbyv1.ErrorCode_ERROR_CODE_NO_SUCH_GAME, Text: "There is no such game."}}})
				continue
			}
			game(env, fakeGame("bob's game", "bob", fakeGameID), 2)
		case *lobbyv1.Envelope_ContinueGame:
			f.note("continue " + m.ContinueGame.Previous)
			reply(env, &lobbyv1.Envelope{Msg: &lobbyv1.Envelope_Ok{Ok: &lobbyv1.Ok{}}})
		case *lobbyv1.Envelope_LeaveGame:
			f.note("leave")
			reply(env, &lobbyv1.Envelope{Msg: &lobbyv1.Envelope_Ok{Ok: &lobbyv1.Ok{}}})
		case *lobbyv1.Envelope_Logout:
			f.mu.Lock()
			delete(f.sessions, fakeSession)
			f.mu.Unlock()
			f.note("logout")
			reply(env, &lobbyv1.Envelope{Msg: &lobbyv1.Envelope_Ok{Ok: &lobbyv1.Ok{}}})
		}
	}
}

// testTLS is a certificate for 127.0.0.1 and a client config trusting it.
func testTLS(t *testing.T) (server, client *tls.Config) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "test"},
		IPAddresses: []net.IP{net.IPv4(127, 0, 0, 1)},
		NotBefore:   time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign, IsCA: true, BasicConstraintsValid: true,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert, _ := x509.ParseCertificate(der)
	pool := x509.NewCertPool()
	pool.AddCert(cert)
	return &tls.Config{Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: key}}},
		&tls.Config{RootCAs: pool}
}

// lobbyRun plays script from the multiplayer screen against lobby f, in
// game directory dir; OPENORION2_NET_SHOTS=dir saves the screen before each step.
func lobbyRun(t *testing.T, f *fakeLobby, dir string, script []lobbyStep) {
	t.Helper()
	step, waited := 0, 0
	err := host.Run(host.Options{Headless: true, Frames: 60 * 60}, func(sys *host.System) error {
		opt := Options{NewGameKeys: true, Seed: 7}
		opt.testHook = func(g *Game) {
			g.Log = func(string, ...any) {}
			g.lobbyTLS = f.tls
			g.settings[settingsMPLink] = 1
			g.currentScreen = ScreenMultiplayer
			g.e.InputHook = func() {
				if step == len(script) {
					sys.Quit()
					return
				}
				if !script[step].when(g) {
					waited = 0
					return
				}
				if waited++; waited < 6 {
					return
				}
				waited = 0
				if d := os.Getenv("OPENORION2_NET_SHOTS"); d != "" {
					g.e.SaveScreenPNG(filepath.Join(d, fmt.Sprintf("%s-%d-%s.png", strings.ReplaceAll(t.Name(), "/", "-"), step, script[step].name)))
				}
				script[step].act(g, sys)
				step++
			}
		}
		return Run(sys, dir, opt)
	})
	if err != nil {
		t.Fatal(err)
	}
	if step < len(script) {
		t.Fatalf("stopped before step %q", script[step].name)
	}
}

func typeLine(s string) func(*Game, *host.System) {
	return func(_ *Game, sys *host.System) {
		for _, c := range []byte(s) {
			sys.PushKey(letterKey(c))
		}
	}
}

func textBoxUp(g *Game) bool  { return g.ub.textBoxing }
func confirmUp(g *Game) bool  { return g.ub.confirming }
func mpScreenUp(g *Game) bool { return g.mpSetup != nil && g.net.box == nil && !g.ub.confirming }

// joinBoxUp is the lobby's JOIN box.
func joinBoxUp(g *Game) bool {
	return g.lobbyJoinScr != nil && g.lobbyJoinScr.joinF > 0 && !g.ub.textBoxing
}

// uniBoxUp is a lobby box with a Unicode line (the game description).
func uniBoxUp(g *Game) bool { return g.net.box != nil && g.net.box.uni != nil && g.net.box.ok > 0 }

func lobbyUp(g *Game) bool {
	return g.lobbyScr != nil && g.net.box == nil && !g.ub.confirming && !g.ub.textBoxing
}
func enterKey(_ *Game, s *host.System) { s.PushKey(letterKey('\r')) }

// TestLobbyLogin: TEN (T) asks for the username and the password (shown
// as '*', pasted the second time), refuses a wrong one, logs in, remembers
// the session; the next visit resumes it; LOGOUT forgets it.
func TestLobbyLogin(t *testing.T) {
	f := startFakeLobby(t)
	dir := scratchGameDir(t, filepath.Join("..", "..", "game"))
	os.WriteFile(filepath.Join(dir, "lobby.yaml"), []byte("server: "+f.addr+"\n"), 0o644)
	var pal [256]engine.Color
	userBox, passBox := boxUp(lobbyHeading), func(g *Game) bool {
		return boxUp(lobbyHeading)(g) && g.net.box.label == "Password"
	}
	lobbyRun(t, f, dir, []lobbyStep{
		// A paste outside an input field is no key: "t" must not open TEN.
		{"multiplayer", mpScreenUp, func(g *Game, sys *host.System) {
			pal = g.e.V.Palette
			sys.PushKey(host.Key{Paste: "t"})
		}},
		{"paste ignored", mpScreenUp, pushKey('t')},
		{"username", userBox, typeLine("alice\r")},
		{"password", passBox, typeLine("wrong horse\r")},
		{"refused", textBoxUp, enterKey},
		{"username again", func(g *Game) bool {
			return userBox(g) && g.net.box.label == "Username" && string(g.rsel.box.text[:5]) == "alice"
		}, enterKey},
		{"password again", passBox, func(_ *Game, sys *host.System) {
			sys.PushKey(host.Key{Paste: "correct horse\n"}) // Ctrl+V; the newline is dropped
		}},
		{"masked", stillTyping(t, "correct horse"), func(g *Game, sys *host.System) {
			if !g.e.FieldMasked(g.rsel.box.inputField) {
				t.Error("the password is not masked")
			}
			enterKey(g, sys)
		}},
		{"logged in", lobbyUp, func(g *Game, sys *host.System) {
			// The lobby leaves the palette alone: the multiplayer screen's.
			if pal != g.e.V.Palette {
				t.Error("the lobby changed the palette")
			}
			clickFieldOf(func(g *Game) int { return g.lobbyScr.backF })(g, sys) // stay logged in
		}},
		{"back", mpScreenUp, pushKey('t')},
		{"resumed", lobbyUp, clickFieldOf(func(g *Game) int { return g.lobbyScr.logoutF })},
		{"back again", mpScreenUp, pushKey('t')},
		{"asks again", userBox, escKeyPress},
		{"left", mpScreenUp, func(*Game, *host.System) {}},
	})
	f.mu.Lock()
	got := append([]string(nil), f.logins...)
	f.mu.Unlock()
	want := []string{"wrong", "login", "resume", "logout"}
	if len(got) != len(want) {
		t.Fatalf("server saw %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("server saw %v, want %v", got, want)
		}
	}
	if r := lobby.LoadRemembered(dir); r.User != "Alice" || r.Session != "" {
		t.Errorf("lobby.session after logout: %+v", r)
	}
	st, err := os.Stat(filepath.Join(dir, "lobby.session"))
	if err != nil || st.Mode().Perm() != 0o600 {
		t.Errorf("lobby.session: %v %v", st, err)
	}
}

// TestLobbyUnreachable: no server: a message, back to the multiplayer
// screen. While it connects, none of the multiplayer screen's buttons show.
func TestLobbyUnreachable(t *testing.T) {
	f := startFakeLobby(t)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skip(err)
	}
	dead := ln.Addr().String()
	ln.Close()
	dir := scratchGameDir(t, filepath.Join("..", "..", "game"))
	os.WriteFile(filepath.Join(dir, "lobby.yaml"), []byte("server: "+dead+"\n"), 0o644)
	lobbyRun(t, f, dir, []lobbyStep{
		{"multiplayer", mpScreenUp, func(g *Game, sys *host.System) {
			// The connecting notice is the first flip without the box's
			// title plate (its top left corner); TEN must be gone too.
			m := g.mpSetup
			px := func(x, y int) byte { return g.e.V.Screen[y*0x280+x] }
			tx, ty := m.x+0x30, m.y+0x18
			bx, by := m.x+0x6e+m.name.Width/2, m.y+0xdc+m.name.Height/2
			title, button := px(tx, ty), px(bx, by)
			g.e.OnToggle = func() {
				if px(tx, ty) != title {
					if px(bx, by) == button {
						t.Error("the multiplayer screen's TEN button shows while connecting")
					}
					g.e.OnToggle = nil
				}
			}
			pushKey('t')(g, sys)
		}},
		{"message", textBoxUp, enterKey},
		{"back", mpScreenUp, func(*Game, *host.System) {}},
	})
}

// TestLobbyLongPassword: the password box takes 64 characters, the
// server's limit, pasted, though only the last of the '*' fit the line.
func TestLobbyLongPassword(t *testing.T) {
	f := startFakeLobby(t)
	dir := scratchGameDir(t, filepath.Join("..", "..", "game"))
	os.WriteFile(filepath.Join(dir, "lobby.yaml"), []byte("server: "+f.addr+"\n"), 0o644)
	long := strings.Repeat("abcdefgh", 9) // 72: the box keeps 64
	lobbyRun(t, f, dir, []lobbyStep{
		{"multiplayer", mpScreenUp, pushKey('t')},
		{"username", boxUp(lobbyHeading), typeLine("alice\r")},
		{"password", func(g *Game) bool { return boxUp(lobbyHeading)(g) && g.net.box.label == "Password" },
			func(_ *Game, sys *host.System) { sys.PushKey(host.Key{Paste: long}) }},
		{"64 kept", stillTyping(t, long[:64]), escKeyPress},
		{"username again", boxUp(lobbyHeading), escKeyPress},
		{"left", mpScreenUp, func(*Game, *host.System) {}},
	})
}

// TestLobbyHome: the lobby's screen shows the snapshot; a line typed and
// Enter goes to the chat; JOIN ID with an unknown id is refused; CREATE asks
// for the description and public or not, copies the new game's id and goes
// on to the new game screen over the lobby's network; leaving that leaves
// the lobby's game and comes back to the lobby without a new login.
func TestLobbyHome(t *testing.T) {
	f := startFakeLobby(t)
	dir := lobbyDir(t, f)
	var clip string
	lobbyRun(t, f, dir, []lobbyStep{
		{"multiplayer", mpScreenUp, pushKey('t')},
		{"lobby", func(g *Game) bool { return lobbyUp(g) && len(g.lobbyScr.conn.State.Chat) == 1 }, func(g *Game, sys *host.System) {
			st := &g.lobbyScr.conn.State
			if len(st.Online) != 2 || len(st.Games) != 1 || st.Games[0].Creator != "bob" {
				t.Errorf("state %+v", st)
			}
			typeLine("hello there")(g, sys)
		}},
		{"typing", func(g *Game) bool { return lobbyUp(g) && g.lobbyScr.line.String() == "hello there" }, enterKey},
		// Typed text in any language (the system's input method's Key.Text).
		{"said", func(g *Game) bool { return lobbyUp(g) && len(g.lobbyScr.conn.State.Chat) == 2 }, func(g *Game, sys *host.System) {
			sys.PushKey(host.Key{Text: "¿Qué? "})
			sys.PushKey(host.Key{Paste: "Привет\n"}) // Ctrl+V; spaces and newlines at its ends dropped
			sys.PushKey(host.Key{Text: "你好"})
			sys.PushKey(host.Key{Code: host.KeyBackspace, Scancode: 42})
			sys.PushKey(letterKey('\r'))
		}},
		{"said unicode", func(g *Game) bool { return lobbyUp(g) && len(g.lobbyScr.conn.State.Chat) == 3 },
			clickFieldOf(func(g *Game) int { return g.lobbyScr.joinF })},
		{"join box", joinBoxUp, func(_ *Game, sys *host.System) {
			sys.PushKey(host.Key{Paste: "not-a-game"})
			sys.PushKey(letterKey('\r'))
		}},
		{"refused", textBoxUp, enterKey},
		{"join box again", joinBoxUp, escKeyPress},
		{"back in", lobbyUp, clickFieldOf(func(g *Game) int { return g.lobbyScr.createF })},
		{"description", uniBoxUp, func(g *Game, sys *host.System) {
			sys.PushKey(host.Key{Code: host.KeyBackspace, Scancode: 42}) // the default name goes at the first backspace
			typeLine("Small galaxy ")(g, sys)
			sys.PushKey(host.Key{Text: "小银河"})
		}},
		{"typed description", func(g *Game) bool { return uniBoxUp(g) && g.net.box.uni.String() == "Small galaxy 小银河" }, enterKey},
		{"public?", confirmUp, pushKey('n')},
		{"private id", textBoxUp, func(g *Game, sys *host.System) {
			clip = sys.Clipboard()
			enterKey(g, sys)
		}},
		{"new game", func(g *Game) bool { return g.currentScreen == ScreenNewGame && g.ng.acceptField > 0 }, func(g *Game, sys *host.System) {
			if _, ok := g.net.network.(*lobby.Network); !ok || g.net.player != "Alice" {
				t.Errorf("network %T, player %q", g.net.network, g.net.player)
			}
			escKeyPress(g, sys)
		}},
		// Back from the game: in the lobby again, on the same connection.
		{"lobby again", lobbyUp, func(g *Game, sys *host.System) {
			if _, ok := g.net.network.(*lobby.Network); ok || g.net.lobbyPlaying {
				t.Error("still on the lobby's network")
			}
			clickFieldOf(func(g *Game) int { return g.lobbyScr.backF })(g, sys)
		}},
		{"out", func(g *Game) bool { return mpScreenUp(g) && g.lobbyConn == nil }, func(*Game, *host.System) {}},
	})
	if clip != fakeNewGameID {
		t.Errorf("clipboard %q", clip)
	}
	f.mu.Lock()
	got := strings.Join(f.logins, "; ")
	f.mu.Unlock()
	want := "resume; say hello there; say ¿Qué? Привет你; join not-a-game; create Small galaxy 小银河 false; leave"
	if got != want {
		t.Errorf("server saw\n %s\nwant\n %s", got, want)
	}
}

// TestLobbyChatScroll: the wheel and PgUp scroll the chat back, End and
// a line said bring it to its end; the chat saved past ChatKeep and
// pages still clamp.
func TestLobbyChatScroll(t *testing.T) {
	f := startFakeLobby(t)
	dir := lobbyDir(t, f)
	const said = 60 // PgUp past the wheel stays inside the chat
	scrolled := func(n int) func(g *Game) bool {
		return func(g *Game) bool { return lobbyUp(g) && g.lobbyScr.scroll == n }
	}
	lobbyRun(t, f, dir, []lobbyStep{
		{"multiplayer", mpScreenUp, pushKey('t')},
		{"lobby", func(g *Game) bool { return lobbyUp(g) && len(g.lobbyScr.conn.State.Chat) == 1 }, func(_ *Game, sys *host.System) {
			for i := range said {
				sys.PushKey(host.Key{Paste: fmt.Sprintf("line %d", i)})
				sys.PushKey(letterKey('\r'))
			}
		}},
		{"said", func(g *Game) bool { return lobbyUp(g) && len(g.lobbyScr.conn.State.Chat) == 1+said }, func(_ *Game, sys *host.System) {
			sys.PushWheel(2)
		}},
		{"wheel", scrolled(2 * lhWheel), func(_ *Game, sys *host.System) {
			sys.PushKey(host.Key{Scancode: 75}) // PgUp
		}},
		{"page up", scrolled(2*lhWheel + lhChatFit - 1), func(_ *Game, sys *host.System) {
			sys.PushKey(host.Key{Scancode: 74}) // Home: the top, clamped
		}},
		{"top", func(g *Game) bool {
			return lobbyUp(g) && g.lobbyScr.scroll == len(lobbyChatLines(g.lobbyScr.conn.State.Chat))-lhChatFit
		}, func(_ *Game, sys *host.System) {
			sys.PushKey(host.Key{Scancode: 77}) // End
		}},
		{"end", scrolled(0), func(_ *Game, sys *host.System) {
			sys.PushWheel(1)
		}},
		{"back a notch", scrolled(lhWheel), func(_ *Game, sys *host.System) {
			sys.PushKey(host.Key{Paste: "bye"})
			sys.PushKey(letterKey('\r'))
		}},
		{"said again", func(g *Game) bool { return scrolled(0)(g) && len(g.lobbyScr.conn.State.Chat) == 2+said }, escKeyPress},
		{"out", mpScreenUp, func(*Game, *host.System) {}},
	})
}

// TestLobbyChatFollow: lines coming in while the chat is scrolled back
// keep it on the lines shown; at the end it follows them.
func TestLobbyChatFollow(t *testing.T) {
	t.Parallel()
	s := &lobbyHomeScr{conn: &lobby.Conn{}}
	st := &s.conn.State
	st.Note("one")
	s.followChat()
	st.Note("two")
	s.followChat()
	if s.scroll != 0 {
		t.Errorf("at the end: scroll %d", s.scroll)
	}
	s.scroll = 1
	st.Note("three")
	st.Note(strings.Repeat("a long line ", 60))
	s.followChat()
	if want := 2 + len(uniWrap(strings.Repeat("a long line ", 60), lhChatWrap)); s.scroll != want {
		t.Errorf("scrolled back: scroll %d, want %d", s.scroll, want)
	}
}

// TestLobbyOnlineScroll: the wheel over who is online scrolls that list,
// not the chat.
func TestLobbyOnlineScroll(t *testing.T) {
	f := startFakeLobby(t)
	f.mu.Lock()
	f.online = 40
	f.mu.Unlock()
	dir := lobbyDir(t, f)
	lobbyRun(t, f, dir, []lobbyStep{
		{"multiplayer", mpScreenUp, pushKey('t')},
		{"lobby", func(g *Game) bool { return lobbyUp(g) && len(g.lobbyScr.conn.State.Online) == 42 }, func(_ *Game, sys *host.System) {
			sys.WarpMouse(lhOnlineX+0x20, lhChatY+0x40)
			sys.PushWheel(-4)
		}},
		{"scrolled", func(g *Game) bool { return lobbyUp(g) && g.lobbyScr.onlineTop == 4 }, func(_ *Game, sys *host.System) {
			sys.PushWheel(-100)
		}},
		{"bottom", func(g *Game) bool { return lobbyUp(g) && g.lobbyScr.onlineTop == 42-lhOnlineFit }, func(g *Game, sys *host.System) {
			if g.lobbyScr.scroll != 0 {
				t.Errorf("chat scrolled %d", g.lobbyScr.scroll)
			}
			escKeyPress(g, sys)
		}},
		{"out", mpScreenUp, func(*Game, *host.System) {}},
	})
}

// TestLobbyPrivate: a line not read from carol (away) opens her tab; a
// click on bob online opens his, a line said there goes to him and his
// answer is read at once; carol's is read when her tab is shown; her x
// closes it; "/w carol" opens it again and says the rest; Esc goes back
// to ALL, then out.
func TestLobbyPrivate(t *testing.T) {
	f := startFakeLobby(t)
	f.mu.Lock()
	f.private = []*lobbyv1.PrivateLine{
		{Id: 7, From: "Alice", To: "carol", Text: "ping"},
		{Id: 9, From: "carol", To: "Alice", Text: "are you there? Ты тут?", Unread: true},
	}
	f.mu.Unlock()
	dir := lobbyDir(t, f)
	scr := func(g *Game) *lobbyHomeScr { return g.lobbyScr }
	tabs := func(want string, cur int) func(g *Game) bool {
		return func(g *Game) bool {
			return lobbyUp(g) && strings.Join(scr(g).tabs, ",") == want && scr(g).cur == cur && scr(g).fieldsSig == scr(g).tabSig()
		}
	}
	lobbyRun(t, f, dir, []lobbyStep{
		{"multiplayer", mpScreenUp, pushKey('t')},
		{"carol's tab", tabs("carol", 0), func(g *Game, sys *host.System) {
			if d := g.lobbyScr.conn.State.Dialog("carol"); d == nil || d.Unread != 1 || len(d.Lines) != 2 {
				t.Errorf("carol's dialog %+v", d)
			}
			clickFieldOf(func(g *Game) int { return scr(g).onlineF[1] })(g, sys) // bob
		}},
		{"bob's tab", tabs("carol,bob", 2), func(g *Game, sys *host.System) {
			typeLine("hi")(g, sys)
			enterKey(g, sys)
		}},
		{"bob answered", func(g *Game) bool {
			d := g.lobbyScr.conn.State.Dialog("bob")
			return lobbyUp(g) && d != nil && len(d.Lines) == 2 && d.Unread == 0
		}, clickFieldOf(func(g *Game) int { return scr(g).tabF[1] })},
		{"carol shown", func(g *Game) bool {
			return tabs("carol,bob", 1)(g) && g.lobbyScr.conn.State.Dialog("carol").Unread == 0
		}, func(g *Game, sys *host.System) {
			g.e.SaveScreenPNG(os.Getenv("OPENORION2_PRIVATE_SHOT"))
			clickFieldOf(func(g *Game) int { return scr(g).closeF[0] })(g, sys)
		}},
		{"carol closed", tabs("bob", 0), func(g *Game, sys *host.System) {
			sys.PushKey(host.Key{Paste: "/w CAROL back soon"})
			enterKey(g, sys)
		}},
		{"carol again", func(g *Game) bool {
			d := g.lobbyScr.conn.State.Dialog("carol")
			return tabs("bob,CAROL", 2)(g) && len(d.Lines) == 3
		}, escKeyPress},
		{"all", tabs("bob,CAROL", 0), escKeyPress},
		{"out", mpScreenUp, func(*Game, *host.System) {}},
	})
	f.mu.Lock()
	got := strings.Join(f.logins, "; ")
	f.mu.Unlock()
	want := "resume; whisper bob hi; read bob 102; read carol 9; whisper carol back soon"
	if got != want {
		t.Errorf("server saw\n %s\nwant\n %s", got, want)
	}
}

// TestLobbyJoinScroll: the JOIN box's list scrolls by the wheel, End,
// PgDn; Down picks the first game and brings it in view; a click picks
// the game on its row, not the list's.
func TestLobbyJoinScroll(t *testing.T) {
	f := startFakeLobby(t)
	f.mu.Lock()
	f.more = 20
	f.mu.Unlock()
	dir := lobbyDir(t, f)
	top := func(n int) func(g *Game) bool {
		return func(g *Game) bool { return joinBoxUp(g) && g.lobbyJoinScr.top == n }
	}
	gameID := func(g *Game, i int) string { return g.lobbyScr.conn.State.Games[i].Id }
	lobbyRun(t, f, dir, []lobbyStep{
		{"multiplayer", mpScreenUp, pushKey('t')},
		{"lobby", func(g *Game) bool { return lobbyUp(g) && len(g.lobbyScr.conn.State.Games) == 21 },
			clickFieldOf(func(g *Game) int { return g.lobbyScr.joinF })},
		{"join box", joinBoxUp, func(_ *Game, sys *host.System) { sys.PushWheel(-3) }},
		{"wheel", top(3), func(_ *Game, sys *host.System) { sys.PushKey(host.Key{Scancode: 77}) }},         // End
		{"end", top(21 - ljRows), func(_ *Game, sys *host.System) { sys.PushKey(host.Key{Scancode: 81}) }}, // Down
		{"down", func(g *Game) bool { return top(0)(g) && g.lobbyJoinScr.sel == gameID(g, 0) }, func(_ *Game, sys *host.System) {
			sys.PushKey(host.Key{Scancode: 78}) // PgDn
		}},
		{"page down", top(ljRows - 1), clickFieldOf(func(g *Game) int { return g.lobbyJoinScr.rowF[2] })},
		{"picked", func(g *Game) bool { return joinBoxUp(g) && g.lobbyJoinScr.sel == gameID(g, ljRows+1) }, escKeyPress},
		{"back", lobbyUp, escKeyPress},
		{"out", mpScreenUp, func(*Game, *host.System) {}},
	})
}

// TestLobbyJoin: a game picked in the list and JOIN go straight to the
// joiner's waiting screen over the lobby's network, without the game list.
func TestLobbyJoin(t *testing.T) {
	f := startFakeLobby(t)
	dir := lobbyDir(t, f)
	lobbyRun(t, f, dir, []lobbyStep{
		{"multiplayer", mpScreenUp, pushKey('t')},
		{"lobby", func(g *Game) bool { return lobbyUp(g) && len(g.lobbyScr.conn.State.Games) == 1 },
			clickFieldOf(func(g *Game) int { return g.lobbyScr.joinF })},
		{"join box", joinBoxUp, clickFieldOf(func(g *Game) int { return g.lobbyJoinScr.rowF[0] })},
		{"picked", func(g *Game) bool { return joinBoxUp(g) && g.lobbyJoinScr.sel == g.lobbyScr.conn.State.Games[0].Id },
			clickFieldOf(func(g *Game) int { return g.lobbyJoinScr.joinF })},
		{"waiting", func(g *Game) bool { return g.net.ep != nil && g.netScr != nil }, func(g *Game, sys *host.System) {
			if _, ok := g.net.network.(*lobby.Network); !ok || g.net.ep.ID() != 2 {
				t.Errorf("network %T, net id %d", g.net.network, g.net.ep.ID())
			}
		}},
	})
	f.mu.Lock()
	got := strings.Join(f.logins, "; ")
	f.mu.Unlock()
	if want := "resume; join " + fakeGameID; got != want {
		t.Errorf("server saw %q, want %q", got, want)
	}
}

// lobbyDir is a game directory whose lobby.yaml names f and whose
// lobby.session resumes.
func lobbyDir(t *testing.T, f *fakeLobby) string {
	dir := scratchGameDir(t, filepath.Join("..", "..", "game"))
	os.WriteFile(filepath.Join(dir, "lobby.yaml"), []byte("server: "+f.addr+"\n"), 0o644)
	lobby.Remembered{User: "Alice", Session: fakeSession}.Save(dir)
	f.mu.Lock()
	f.sessions[fakeSession] = true
	f.mu.Unlock()
	return dir
}

// TestLobbyLoad: LOAD makes the lobby's game for a saved one and goes to
// the network save picker; with no network save there it is said, the game
// left, and the player is back in the lobby.
func TestLobbyLoad(t *testing.T) {
	f := startFakeLobby(t)
	dir := lobbyDir(t, f)
	lobbyRun(t, f, dir, []lobbyStep{
		{"multiplayer", mpScreenUp, pushKey('t')},
		{"lobby", lobbyUp, clickFieldOf(func(g *Game) int { return g.lobbyScr.loadF })},
		{"description", uniBoxUp, func(g *Game, sys *host.System) {
			if v := g.net.box.uni.String(); v != "Alice's saved game" {
				t.Errorf("default description %q", v)
			}
			enterKey(g, sys)
		}},
		{"public?", confirmUp, pushKey('y')},
		{"no saves", func(g *Game) bool { return g.ub.textBoxing || g.ub.exploding || g.ub.confirming }, func(g *Game, sys *host.System) {
			if _, ok := g.net.network.(*lobby.Network); !ok {
				t.Errorf("network %T", g.net.network)
			}
			enterKey(g, sys)
		}},
		{"lobby again", lobbyUp, clickFieldOf(func(g *Game) int { return g.lobbyScr.backF })},
		{"out", mpScreenUp, func(*Game, *host.System) {}},
	})
	f.mu.Lock()
	got := strings.Join(f.logins, "; ")
	f.mu.Unlock()
	if want := "resume; create Alice's saved game true; leave"; got != want {
		t.Errorf("server saw %q, want %q", got, want)
	}
}

// TestLobbyJoinByID: the JOIN box takes a whole game id, typed or pasted
// (its line shows the end, steady while the cursor blinks), and joins it.
func TestLobbyJoinByID(t *testing.T) {
	for _, paste := range []bool{false, true} {
		t.Run(map[bool]string{false: "typed", true: "pasted"}[paste], func(t *testing.T) {
			f := startFakeLobby(t)
			dir := lobbyDir(t, f)
			var frames []string
			lobbyRun(t, f, dir, []lobbyStep{
				{"multiplayer", mpScreenUp, pushKey('t')},
				{"lobby", lobbyUp, clickFieldOf(func(g *Game) int { return g.lobbyScr.joinF })},
				{"join box", joinBoxUp, func(g *Game, sys *host.System) {
					if paste {
						sys.PushKey(host.Key{Paste: fakeGameID})
						return
					}
					typeLine(fakeGameID)(g, sys)
				}},
				// The line stays put while the cursor blinks: the left two
				// thirds of the field are the same in every frame.
				{"whole id", func(g *Game) bool { return joinBoxUp(g) && g.lobbyJoinScr.id.String() == fakeGameID }, func(g *Game, sys *host.System) {
					x1, y1, y2 := ljX1+10, ljIDY, ljIDY+uniLineH-1
					x2 := x1 + uniWidth(fakeGameID)*3/2 // the text's two thirds: the cursor after it blinks
					g.e.OnToggle = func() {
						var b []byte
						for y := y1; y <= y2; y++ {
							b = append(b, g.e.V.Screen[y*0x280+x1:y*0x280+x1+(x2-x1)*2/3]...)
						}
						frames = append(frames, string(b))
					}
				}},
				{"steady", func(*Game) bool { return len(frames) >= 12 }, func(g *Game, sys *host.System) {
					g.e.OnToggle = nil
					for i, f := range frames {
						if f != frames[0] {
							t.Errorf("frame %d of the id line differs from frame 0", i) // not Fatal: the game's goroutine
							break
						}
					}
					enterKey(g, sys)
				}},
				{"waiting", func(g *Game) bool { return g.net.ep != nil && g.netScr != nil }, func(*Game, *host.System) {}},
			})
			f.mu.Lock()
			got := strings.Join(f.logins, "; ")
			f.mu.Unlock()
			if want := "resume; join " + fakeGameID; got != want {
				t.Errorf("server saw %q, want %q", got, want)
			}
		})
	}
}
