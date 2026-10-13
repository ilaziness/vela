package bookmark

import (
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/ilaziness/vela/internal/database"
	"github.com/ilaziness/vela/internal/secret"
)

func TestPrepareTestSwitchToStoredKeyKeepsPassword(t *testing.T) {
	svc, id := testBookmarkService(t, "/keys/id_ed25519", "key-pass")
	prep, err := svc.PrepareTest(Bookmark{
		ID: id, Host: "h", Port: 22, User: "root",
		Password: PasswordMask, PrivateKey: "", PrivateKeyPassword: "",
		SshKeyID: "key-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	ep := prep.Endpoint
	if ep.Password != "login-secret" {
		t.Fatalf("password = %q", ep.Password)
	}
	if ep.Key != "" || ep.KeyPassword != "" {
		t.Fatalf("file key leaked: path=%q pass=%q", ep.Key, ep.KeyPassword)
	}
	if ep.KeyPEM != "PEM" {
		t.Fatalf("key pem = %q", ep.KeyPEM)
	}
}

func TestPrepareTestClearedPasswordStaysEmpty(t *testing.T) {
	svc, id := testBookmarkService(t, "/keys/id_ed25519", "key-pass")
	prep, err := svc.PrepareTest(Bookmark{
		ID: id, Host: "h", Port: 22, User: "root",
		Password: "", PrivateKey: "", PrivateKeyPassword: "",
		SshKeyID: "key-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	ep := prep.Endpoint
	if ep.Password != "" {
		t.Fatalf("password = %q", ep.Password)
	}
	if ep.KeyPEM != "PEM" {
		t.Fatalf("key pem = %q", ep.KeyPEM)
	}
}

func TestPrepareTestUnchangedFileKeepsKeyPassword(t *testing.T) {
	svc, id := testBookmarkService(t, "/keys/id_ed25519", "key-pass")
	prep, err := svc.PrepareTest(Bookmark{
		ID: id, Host: "h", Port: 22, User: "root",
		Password: PasswordMask, PrivateKey: "/keys/id_ed25519", PrivateKeyPassword: PasswordMask,
		ProxyJumpID: "jump-new",
	})
	if err != nil {
		t.Fatal(err)
	}
	if prep.JumpID != "jump-new" {
		t.Fatalf("jump = %q", prep.JumpID)
	}
	ep := prep.Endpoint
	if ep.Password != "login-secret" || ep.KeyPassword != "key-pass" || ep.Key != "/keys/id_ed25519" {
		t.Fatalf("endpoint = %+v", ep)
	}
	if ep.KeyPEM != "" {
		t.Fatal("unexpected stored key")
	}
}

func TestPrepareTestKeepsFormCertificate(t *testing.T) {
	svc, id := testBookmarkService(t, "/keys/id_ed25519", "key-pass")
	prep, err := svc.PrepareTest(Bookmark{
		ID: id, Host: "h", Port: 22, User: "root",
		Password: PasswordMask, PrivateKey: "/keys/id_ed25519", PrivateKeyPassword: PasswordMask,
		Certificate: "/keys/user-cert.pub",
	})
	if err != nil {
		t.Fatal(err)
	}
	ep := prep.Endpoint
	if ep.Certificate != "/keys/user-cert.pub" {
		t.Fatalf("certificate = %q", ep.Certificate)
	}
	if ep.Password != "login-secret" || ep.KeyPassword != "key-pass" {
		t.Fatalf("endpoint = %+v", ep)
	}
}

func TestPrepareTestRestoresProxyPassword(t *testing.T) {
	svc, id := testBookmarkService(t, "/keys/id_ed25519", "key-pass")
	prep, err := svc.PrepareTest(Bookmark{
		ID: id, Host: "h", Port: 22, User: "root",
		Password: PasswordMask, PrivateKey: "/keys/id_ed25519", PrivateKeyPassword: PasswordMask,
		ProxyMode: "custom", ProxyType: "socks5", ProxyHost: "127.0.0.1", ProxyPort: 1080,
		ProxyUser: "pu", ProxyPassword: PasswordMask,
	})
	if err != nil {
		t.Fatal(err)
	}
	if prep.ProxyPassword != "proxy-secret" {
		t.Fatalf("proxy password = %q", prep.ProxyPassword)
	}
}

func testBookmarkService(t *testing.T, keyPath, keyPass string) (*Service, string) {
	t.Helper()
	db := database.NewDatabase(t.TempDir(), zap.NewNop())
	if err := db.Initialize(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.BookmarkRepo.InsertGroup(&database.BookmarkGroupDB{Name: "默认书签"}); err != nil {
		t.Fatal(err)
	}
	group, err := db.BookmarkRepo.GetGroupByName("默认书签")
	if err != nil {
		t.Fatal(err)
	}
	login, err := secret.Encrypt("master", "login-secret")
	if err != nil {
		t.Fatal(err)
	}
	keyPassword, err := secret.Encrypt("master", keyPass)
	if err != nil {
		t.Fatal(err)
	}
	proxyPass, err := secret.Encrypt("master", "proxy-secret")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	const id = "bm-1"
	if err := db.BookmarkRepo.InsertBookmark(&database.BookmarkDB{
		ID: id, GroupID: group.ID, Title: "t", Host: "h", Port: 22, User: "root",
		Password: login, PrivateKey: keyPath, PrivateKeyPassword: keyPassword,
		ProxyJumpID: "jump-old",
		ProxyMode:   "custom", ProxyType: "socks5", ProxyHost: "127.0.0.1", ProxyPort: 1080,
		ProxyUser: "pu", ProxyPassword: proxyPass,
		CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	svc := New(zap.NewNop(), db, func(string) (string, error) { return "master", nil }, nil, nil, func(keyID string) (string, error) {
		if keyID != "key-1" {
			t.Fatalf("unexpected key id %s", keyID)
		}
		return "PEM", nil
	})
	return svc, id
}
