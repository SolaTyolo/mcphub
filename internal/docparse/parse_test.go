package docparse

import "testing"

func TestSupported(t *testing.T) {
	for _, name := range []string{
		"report.xlsx", "notes.docx", "slides.pptx",
		"doc.pdf", "data.json", "sheet.csv",
	} {
		if !Supported(name) {
			t.Fatalf("expected supported: %s", name)
		}
	}
	if Supported("file.bin") {
		t.Fatal("binary should not be supported")
	}
}
