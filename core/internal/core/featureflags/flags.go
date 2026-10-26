// Package featureflags carrega e expõe as funcionalidades opcionais do TON-618.
//
// O estado é persistido na tabela `settings` e cacheado em um Manager, para que
// o cabeçalho (Navbar) possa decidir no servidor o que renderizar sem uma query
// por requisição. Hoje só existe uma flag — Rotina — mas o pacote centraliza o
// padrão para as próximas.
package featureflags

import (
	"context"
	"strings"
	"sync"
)

// RotinaEnabledKey é a chave em `settings` que guarda o estado da Rotina.
const RotinaEnabledKey = "rotina_enabled"

// RotinaNativeTimelineKey guarda qual renderer a Rotina usa: "1" (padrão) para
// a linha do tempo nativa, "0" para o Mermaid legado. A chave existe para
// permitir comparar os dois lado a lado antes de remover o vendor do Mermaid.
const RotinaNativeTimelineKey = "rotina_timeline_native"

// SettingStore é o subconjunto do Store usado pelo Manager (permite testar sem
// SQLite).
type SettingStore interface {
	GetSetting(key string) (string, error)
	SetSetting(key, value string) error
}

// Flags é o retrato das funcionalidades opcionais no momento da requisição.
type Flags struct {
	Rotina bool
	// RotinaNativeTimeline escolhe o renderer da linha do tempo da Rotina.
	// true = nativo (sem Mermaid); false = Mermaid legado.
	RotinaNativeTimeline bool
}

// Default é o estado de fábrica: tudo habilitado. Uma instalação que nunca
// tocou na flag continua vendo a Rotina no cabeçalho, já com a linha do tempo
// nativa (o Mermaid fica só como opção de comparação).
func Default() Flags { return Flags{Rotina: true, RotinaNativeTimeline: true} }

type flagsContextKey struct{}

// WithFlags injeta o retrato das flags no contexto da requisição.
func WithFlags(ctx context.Context, f Flags) context.Context {
	return context.WithValue(ctx, flagsContextKey{}, f)
}

// FromContext devolve as flags da requisição; fora de uma requisição (ou em
// testes) cai no padrão habilitado.
func FromContext(ctx context.Context) Flags {
	if ctx != nil {
		if f, ok := ctx.Value(flagsContextKey{}).(Flags); ok {
			return f
		}
	}
	return Default()
}

// ParseBool interpreta o valor persistido em `settings`. Valores vazios (nunca
// configurado) e desconhecidos usam o fallback — a flag só desliga com um valor
// explícito de desligamento.
func ParseBool(raw string, fallback bool) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "":
		return fallback
	case "1", "true", "on", "yes", "enabled", "ligado", "sim":
		return true
	case "0", "false", "off", "no", "disabled", "desligado", "nao", "não":
		return false
	default:
		return fallback
	}
}

// BoolString converte um booleano para o formato persistido ("1"/"0").
func BoolString(v bool) string {
	if v {
		return "1"
	}
	return "0"
}

// Manager mantém em cache o estado atual e invalida o cache em cada escrita.
// É seguro para uso concorrente (RWMutex).
type Manager struct {
	store SettingStore

	mu    sync.RWMutex
	flags Flags
}

// NewManager cria o Manager já com o estado lido do banco. Se a leitura falhar
// (ou o store for nil), mantém o padrão habilitado — uma falha de settings não
// pode sumir com a funcionalidade.
func NewManager(store SettingStore) *Manager {
	m := &Manager{store: store, flags: Default()}
	m.Reload()
	return m
}

// Reload relê a flag do banco e atualiza o cache. Devolve o estado aplicado.
func (m *Manager) Reload() Flags {
	flags := Default()
	if m != nil && m.store != nil {
		if raw, err := m.store.GetSetting(RotinaEnabledKey); err == nil {
			flags.Rotina = ParseBool(raw, true)
		}
		if raw, err := m.store.GetSetting(RotinaNativeTimelineKey); err == nil {
			flags.RotinaNativeTimeline = ParseBool(raw, true)
		}
	}
	if m != nil {
		m.mu.Lock()
		m.flags = flags
		m.mu.Unlock()
	}
	return flags
}

// Get devolve o retrato atual (cópia, seguro para guardar no contexto).
func (m *Manager) Get() Flags {
	if m == nil {
		return Default()
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.flags
}

// RotinaEnabled é o atalho booleano usado nos handlers.
func (m *Manager) RotinaEnabled() bool { return m.Get().Rotina }

// SetRotinaEnabled persiste a flag e só então atualiza o cache. Se a escrita
// falhar, o estado em memória permanece o anterior (nada de UI mentindo).
func (m *Manager) SetRotinaEnabled(enabled bool) error {
	if m == nil || m.store == nil {
		return nil
	}
	if err := m.store.SetSetting(RotinaEnabledKey, BoolString(enabled)); err != nil {
		return err
	}
	m.mu.Lock()
	m.flags.Rotina = enabled
	m.mu.Unlock()
	return nil
}

// SetRotinaNativeTimeline escolhe o renderer da linha do tempo. Mesmo contrato
// do SetRotinaEnabled: só atualiza o cache depois de persistir.
func (m *Manager) SetRotinaNativeTimeline(native bool) error {
	if m == nil || m.store == nil {
		return nil
	}
	if err := m.store.SetSetting(RotinaNativeTimelineKey, BoolString(native)); err != nil {
		return err
	}
	m.mu.Lock()
	m.flags.RotinaNativeTimeline = native
	m.mu.Unlock()
	return nil
}
