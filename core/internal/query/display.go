package query

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"ton618/core/internal/core/domain"
)

// ---------------------------------------------------------------------------
// Apresentação
//
// Estes helpers existem para o template do painel não precisar conhecer as
// regras internas (e para a lista de colunas válidas ter um dono só). Nada aqui
// participa da avaliação do `onde:`.
// ---------------------------------------------------------------------------

// OpenURL devolve o destino do clique na linha, com a MESMA regra do resto do
// app (nota → editor, pdf/epub/desenho → sua rota).
func (r Row) OpenURL() string {
	url, _ := domain.NoteOpenTarget(domain.NoteType(r.Classe), r.Caminho)
	return url
}

// NovaAba diz se o destino deve abrir em outra aba (binários/visualizadores).
func (r Row) NovaAba() bool {
	_, blank := domain.NoteOpenTarget(domain.NoteType(r.Classe), r.Caminho)
	return blank
}

// MtimeRel é o "há quanto tempo" da última modificação (texto curto).
func (r Row) MtimeRel() string { return tempoRel(r.Mtime) }

// InteracaoRel é o "há quanto tempo" da última interação.
func (r Row) InteracaoRel() string { return tempoRel(r.Interacao) }

// Coluna devolve o texto de exibição de uma coluna de `mostrar: tabela`.
// A lista do que é aceito vive em processor.QueryColunas (validada no parser).
func (r Row) Coluna(nome string) string {
	switch nome {
	case "titulo":
		return r.Titulo
	case "caminho", "arquivo":
		return r.Caminho
	case "pasta":
		return r.Pasta
	case "classe":
		return r.Classe
	case "pai":
		return r.Pai
	case "tags":
		return strings.Join(r.Tags, ", ")
	case "n_tags":
		return strconv.Itoa(len(r.Tags))
	case "filhas":
		return strconv.Itoa(len(r.Filhas))
	case "mtime", "modif_ha_dias":
		if nome == "modif_ha_dias" {
			return diasTexto(r.Mtime)
		}
		return dataTexto(r.Mtime)
	case "interacao":
		return dataTexto(r.Interacao)
	case "interacao_ha_dias":
		return diasTexto(r.Interacao)
	case "pai_resolvido":
		if r.PaiResolvido {
			return "sim"
		}
		return "não"
	case "citada_por":
		return strconv.Itoa(r.CitadaPor)
	case "tarefas_abertas":
		return strconv.Itoa(r.TarefasAbertas)
	case "popularidade":
		return strconv.Itoa(r.Popularidade)
	case "texto":
		return r.Texto
	case "nota_titulo":
		return r.Titulo
	case "marcador":
		return r.Marcador
	case "estado":
		return r.Estado
	case "secao":
		return r.Secao
	case "linha":
		return strconv.Itoa(r.Linha)
	case "criado":
		return dataTexto(r.Criado)
	}
	return ""
}

// ContagemTexto é o número grande da apresentação `contagem`.
func (r Result) ContagemTexto() string { return strconv.Itoa(r.Total) }

// Vazio diz se o resultado não trouxe nada (para o cartão explicar em vez de
// mostrar uma caixa branca).
func (r Result) Vazio() bool { return r.Erro == "" && len(r.Rows) == 0 }

// Rodape descreve o que foi truncado/limitado — transparência sobre o teto de
// candidatos e sobre o `limite:` (nunca mentir sobre o que foi avaliado).
func (r Result) Rodape() string {
	var partes []string
	if r.Capado {
		partes = append(partes, fmt.Sprintf("avaliadas as %d primeiras de %d", r.Candidatos, r.CandidatosTotais))
	}
	if r.Truncado {
		partes = append(partes, fmt.Sprintf("mostrando %d de %d", len(r.Rows), r.Total))
	}
	return strings.Join(partes, " · ")
}

func tempoRel(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	dias := int(time.Since(t).Hours() / 24)
	switch {
	case dias <= 0:
		return "hoje"
	case dias == 1:
		return "ontem"
	case dias < 30:
		return fmt.Sprintf("há %d dias", dias)
	case dias < 365:
		return fmt.Sprintf("há %d meses", dias/30)
	default:
		return fmt.Sprintf("há %d anos", dias/365)
	}
}

func diasTexto(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return strconv.Itoa(int(time.Since(t).Hours() / 24))
}

func dataTexto(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Local().Format("02/01/2006")
}
