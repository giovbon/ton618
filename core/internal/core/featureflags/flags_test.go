package featureflags

import (
	"sync"
	"testing"
)

type memSettings struct {
	mu   sync.Mutex
	vals map[string]string
}

func newMemSettings() *memSettings { return &memSettings{vals: map[string]string{}} }

func (m *memSettings) GetSetting(key string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.vals[key], nil
}

func (m *memSettings) SetSetting(key, value string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.vals[key] = value
	return nil
}

func TestParseBool(t *testing.T) {
	cases := []struct {
		raw      string
		fallback bool
		want     bool
	}{
		{"", true, true},
		{"", false, false},
		{"1", false, true},
		{"true", false, true},
		{"ON", false, true},
		{"sim", false, true},
		{"0", true, false},
		{"false", true, false},
		{"off", true, false},
		{"não", true, false},
		{"qualquer", true, true},
	}
	for _, c := range cases {
		if got := ParseBool(c.raw, c.fallback); got != c.want {
			t.Errorf("ParseBool(%q, %v) = %v, want %v", c.raw, c.fallback, got, c.want)
		}
	}
}

func TestManager_DefaultEnabled(t *testing.T) {
	m := NewManager(newMemSettings())
	if !m.RotinaEnabled() {
		t.Fatal("sem configuração, a Rotina deve vir habilitada")
	}
}

func TestManager_SetPersistsAndReloads(t *testing.T) {
	store := newMemSettings()
	m := NewManager(store)

	if err := m.SetRotinaEnabled(false); err != nil {
		t.Fatalf("SetRotinaEnabled: %v", err)
	}
	if m.RotinaEnabled() {
		t.Fatal("cache deveria estar desligado")
	}
	if store.vals[RotinaEnabledKey] != "0" {
		t.Fatalf("valor persistido = %q, quer \"0\"", store.vals[RotinaEnabledKey])
	}

	// Um novo Manager lê o mesmo banco e continua desligado.
	if NewManager(store).RotinaEnabled() {
		t.Fatal("novo Manager deveria ler a flag desligada")
	}

	if err := m.SetRotinaEnabled(true); err != nil {
		t.Fatalf("SetRotinaEnabled(true): %v", err)
	}
	if !NewManager(store).RotinaEnabled() {
		t.Fatal("novo Manager deveria ler a flag ligada")
	}
}

func TestFromContext_Default(t *testing.T) {
	if !FromContext(nil).Rotina {
		t.Fatal("contexto nil deve cair no padrão habilitado")
	}
	if !FromContext(t.Context()).Rotina {
		t.Fatal("contexto sem flags deve cair no padrão habilitado")
	}

	ctx := WithFlags(t.Context(), Flags{Rotina: false})
	if FromContext(ctx).Rotina {
		t.Fatal("flags injetadas não foram lidas")
	}
}
