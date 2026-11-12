package api

import "testing"

func TestBindIPRejectsAmbiguousListenerValues(t *testing.T) {
	s := &Server{}
	for _, value := range []string{"127.0.0.1", "::1", "", "0.0.0.0"} {
		if err := s.BindIP(value); err != nil {
			t.Fatalf("literal IP %q: %v", value, err)
		}
	}
	for _, value := range []string{"localhost", "127.0.0.1:8080", "[::1]", "::1%lo", " 127.0.0.1", "127.0.0.1\n"} {
		if err := s.BindIP(value); err == nil {
			t.Fatalf("accepted ambiguous listener %q", value)
		}
	}
}
