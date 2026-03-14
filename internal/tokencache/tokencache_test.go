package tokencache

import "testing"

func TestFileStoreSaveLoadClear(t *testing.T) {
	store := newFileStore(t.TempDir())
	id := Identity{AuthURL: "https://auth.ecobee.com/oauth/token", ClientID: "client-a", Email: "user@example.com"}
	token := CachedToken{AccessToken: "access", RefreshToken: "refresh"}

	if err := store.Save(id, token); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Load(id)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.AccessToken != "access" || loaded.RefreshToken != "refresh" {
		t.Fatalf("unexpected token loaded: %#v", loaded)
	}
	if err := store.Clear(id); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load(id); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound after clear, got %v", err)
	}
}

func TestCacheIsolationByClientID(t *testing.T) {
	store := &Manager{fallback: newFileStore(t.TempDir())}
	idA := Identity{AuthURL: "https://auth.ecobee.com/oauth/token", ClientID: "client-a", Email: "user@example.com"}
	idB := Identity{AuthURL: "https://auth.ecobee.com/oauth/token", ClientID: "client-b", Email: "user@example.com"}
	if err := store.Save(idA, CachedToken{AccessToken: "token-a"}); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(idB, CachedToken{AccessToken: "token-b"}); err != nil {
		t.Fatal(err)
	}
	tokenA, err := store.Load(idA)
	if err != nil {
		t.Fatal(err)
	}
	tokenB, err := store.Load(idB)
	if err != nil {
		t.Fatal(err)
	}
	if tokenA.AccessToken == tokenB.AccessToken {
		t.Fatalf("expected separate cache entries by client ID")
	}
}
