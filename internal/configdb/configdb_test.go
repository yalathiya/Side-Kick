package configdb

import (
	"os"
	"path/filepath"
	"testing"
)

func tempDB(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	return filepath.Join(dir, "test.db")
}

func TestOpen_CreatesTablesAndLoads(t *testing.T) {
	db, err := Open(tempDB(t), 60)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	snap := db.Current()
	if snap == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if len(snap.Config) != 0 {
		t.Errorf("expected empty config, got %d entries", len(snap.Config))
	}
	if len(snap.RateLimits) != 0 {
		t.Errorf("expected no rate limits, got %d", len(snap.RateLimits))
	}
}

func TestSetConfig_AndGetConfig(t *testing.T) {
	db, err := Open(tempDB(t), 60)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	if err := db.SetConfig("app_name", "sidekick"); err != nil {
		t.Fatalf("SetConfig failed: %v", err)
	}

	if v := db.GetConfig("app_name"); v != "sidekick" {
		t.Errorf("expected sidekick, got %s", v)
	}
}

func TestSetConfig_Upsert(t *testing.T) {
	db, err := Open(tempDB(t), 60)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	db.SetConfig("key", "val1")
	db.SetConfig("key", "val2")

	if v := db.GetConfig("key"); v != "val2" {
		t.Errorf("expected val2 after upsert, got %s", v)
	}
}

func TestDeleteConfig(t *testing.T) {
	db, err := Open(tempDB(t), 60)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	db.SetConfig("temp", "value")
	if err := db.DeleteConfig("temp"); err != nil {
		t.Fatalf("DeleteConfig failed: %v", err)
	}

	if v := db.GetConfig("temp"); v != "" {
		t.Errorf("expected empty after delete, got %s", v)
	}
}

func TestAddRateLimit_AndQuery(t *testing.T) {
	db, err := Open(tempDB(t), 60)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	if err := db.AddRateLimit("/api/v1/users", 100, 60); err != nil {
		t.Fatalf("AddRateLimit failed: %v", err)
	}

	rules := db.GetRateLimits()
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
	if rules[0].Path != "/api/v1/users" {
		t.Errorf("expected path /api/v1/users, got %s", rules[0].Path)
	}
	if rules[0].Limit != 100 {
		t.Errorf("expected limit 100, got %d", rules[0].Limit)
	}
	if rules[0].WindowSeconds != 60 {
		t.Errorf("expected window 60, got %d", rules[0].WindowSeconds)
	}
}

func TestRateLimitForPath(t *testing.T) {
	db, err := Open(tempDB(t), 60)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	db.AddRateLimit("/api/fast", 1000, 1)
	db.AddRateLimit("/api/slow", 5, 60)

	rule := db.RateLimitForPath("/api/fast")
	if rule == nil {
		t.Fatal("expected rule for /api/fast")
	}
	if rule.Limit != 1000 {
		t.Errorf("expected 1000, got %d", rule.Limit)
	}

	if db.RateLimitForPath("/api/missing") != nil {
		t.Error("expected nil for non-existent path")
	}
}

func TestRemoveRateLimit(t *testing.T) {
	db, err := Open(tempDB(t), 60)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	db.AddRateLimit("/temp", 10, 10)
	rules := db.GetRateLimits()
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}

	if err := db.RemoveRateLimit(rules[0].ID); err != nil {
		t.Fatalf("RemoveRateLimit failed: %v", err)
	}

	if len(db.GetRateLimits()) != 0 {
		t.Error("expected 0 rules after removal")
	}
}

func TestReload_ReflectsExternalChanges(t *testing.T) {
	path := tempDB(t)
	db, err := Open(path, 60)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	// Write directly via SQL (simulating external tool)
	db.db.Exec(`INSERT INTO config (key, value) VALUES ('external', 'data')`)

	// Before reload — not visible
	if v := db.GetConfig("external"); v != "" {
		t.Errorf("expected empty before reload, got %s", v)
	}

	// After reload
	db.Reload()
	if v := db.GetConfig("external"); v != "data" {
		t.Errorf("expected data after reload, got %s", v)
	}
}

func TestOpen_InvalidPath(t *testing.T) {
	_, err := Open("/nonexistent/dir/test.db", 60)
	if err == nil {
		t.Error("expected error for invalid path")
	}
}

func TestSnapshot_LoadedAt(t *testing.T) {
	db, err := Open(tempDB(t), 60)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	snap := db.Current()
	if snap.LoadedAt.IsZero() {
		t.Error("expected non-zero LoadedAt")
	}
}

func TestOpen_PersistsBetweenConnections(t *testing.T) {
	path := tempDB(t)

	// First connection: write data
	db1, _ := Open(path, 60)
	db1.SetConfig("persist", "yes")
	db1.Close()

	// Second connection: read back
	db2, err := Open(path, 60)
	if err != nil {
		t.Fatalf("failed to reopen: %v", err)
	}
	defer db2.Close()

	if v := db2.GetConfig("persist"); v != "yes" {
		t.Errorf("expected 'yes' after reopen, got %s", v)
	}
}

func TestOpen_NonExistentDir(t *testing.T) {
	// Path with non-existent parent directory
	path := filepath.Join(os.TempDir(), "nonexistent_sidekick_test", "test.db")
	defer os.RemoveAll(filepath.Dir(path))

	_, err := Open(path, 60)
	if err == nil {
		t.Error("expected error for non-existent directory")
	}
}
