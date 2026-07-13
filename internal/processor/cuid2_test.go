package processor

import "testing"

func TestGenerateCUID2_Output(t *testing.T) {
	for i := 0; i < 5; i++ {
		name := GenerateCUID2()
		t.Logf("name: %s", name)
		if len(name) < 5 {
			t.Errorf("name too short: %s", name)
		}
	}
}
