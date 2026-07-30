package processor

import "testing"

func TestGenerateCUID2_Output(t *testing.T) {
	for i := 0; i < 5; i++ {
		name := GenerateCUID2()
		t.Logf("name: %s", name)
		if len(name) < 5 {
			t.Errorf("name too short: %s", name)
		}
		if !IsCUID2(name) {
			t.Errorf("GenerateCUID2() output %s deve ser reconhecido por IsCUID2", name)
		}
	}
}

func TestIsCUID2_And_IsDraftName(t *testing.T) {
	if !IsCUID2("notes/impar-lareira-99.md") {
		t.Errorf("esperado true para impar-lareira-99")
	}
	if !IsDraftName("notes/impar-lareira-99.md") {
		t.Errorf("esperado IsDraftName true para impar-lareira-99")
	}
	if IsCUID2("notes/nota-que-nao-existe.md") {
		t.Errorf("esperado false para nota-que-nao-existe")
	}
	if IsDraftName("notes/nota-que-nao-existe.md") {
		t.Errorf("esperado IsDraftName false para nota-que-nao-existe")
	}
}
