package sftp

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

func TestDecideResume(t *testing.T) {
	tests := []struct {
		name       string
		dest, src  int64
		wantAction ResumeAction
		wantOff    int64
	}{
		{"no dest", 0, 100, ResumeCopy, 0},
		{"missing dest", -1, 100, ResumeCopy, 0},
		{"partial", 40, 100, ResumeContinue, 40},
		{"complete", 100, 100, ResumeSkip, 100},
		{"dest larger", 120, 100, ResumeCopy, 0},
		{"empty source complete", 0, 0, ResumeCopy, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			act, off := DecideResume(tc.dest, tc.src)
			if act != tc.wantAction || off != tc.wantOff {
				t.Fatalf("DecideResume(%d,%d)=(%v,%d), want (%v,%d)",
					tc.dest, tc.src, act, off, tc.wantAction, tc.wantOff)
			}
		})
	}
}

func TestOverwriteUploadDoesNotCallDecideResume(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "manager.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name == nil || fn.Name.Name != "uploadFileReplace" || fn.Body == nil {
			continue
		}
		found = true
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			switch v := n.(type) {
			case *ast.Ident:
				if v.Name == "DecideResume" || v.Name == "resumePlan" {
					t.Fatalf("uploadFileReplace references %s", v.Name)
				}
			case *ast.SelectorExpr:
				if v.Sel != nil && (v.Sel.Name == "DecideResume" || v.Sel.Name == "resumePlan") {
					t.Fatalf("uploadFileReplace references %s", v.Sel.Name)
				}
			}
			return true
		})
	}
	if !found {
		t.Fatal("uploadFileReplace not found")
	}
}

func TestEditStagingRemotePath(t *testing.T) {
	final := "/home/user/notes.txt"
	staging := editStagingRemotePath(final)
	if staging == final {
		t.Fatal("staging path must differ from the final remote path")
	}
	if staging != "/home/user/.notes.txt.vela-edit" {
		t.Fatalf("staging=%q", staging)
	}
}
