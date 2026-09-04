package deploy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseProcfile(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		useFile   bool // if false, use a non-existent path
		wantErr   bool
		errSubstr string // substring expected in error message
		want      []ProcEntry
	}{
		{
			name:    "valid_single_process",
			content: "web: npm start\n",
			useFile: true,
			wantErr: false,
			want: []ProcEntry{
				{Name: "web", Command: "npm start"},
			},
		},
		{
			name:    "valid_multiple_processes",
			content: "web: npm start\nworker: python worker.py\n",
			useFile: true,
			wantErr: false,
			want: []ProcEntry{
				{Name: "web", Command: "npm start"},
				{Name: "worker", Command: "python worker.py"},
			},
		},
		{
			name: "comments_and_blank_lines",
			content: `# This is a comment

web: node app.js
   
# Another comment
`,
			useFile: true,
			wantErr: false,
			want: []ProcEntry{
				{Name: "web", Command: "node app.js"},
			},
		},
		{
			name:      "missing_file",
			useFile:   false,
			wantErr:   true,
			errSubstr: "opening Procfile",
			want:      nil,
		},
		{
			name:      "empty_file",
			content:   "",
			useFile:   true,
			wantErr:   true,
			errSubstr: "no valid entries",
			want:      nil,
		},
		{
			name:      "invalid_format_no_colon",
			content:   "web npm start\n",
			useFile:   true,
			wantErr:   true,
			errSubstr: "invalid format",
			want:      nil,
		},
		{
			name:      "empty_command",
			content:   "web:\n",
			useFile:   true,
			wantErr:   true,
			errSubstr: "empty",
			want:      nil,
		},
		{
			name:    "command_with_colons",
			content: "web: python app.py --bind 0.0.0.0:8000\n",
			useFile: true,
			wantErr: false,
			want: []ProcEntry{
				{Name: "web", Command: "python app.py --bind 0.0.0.0:8000"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var path string
			if tt.useFile {
				dir := t.TempDir()
				path = filepath.Join(dir, "Procfile")
				if err := os.WriteFile(path, []byte(tt.content), 0644); err != nil {
					t.Fatalf("failed to write temporary Procfile: %v", err)
				}
			} else {
				path = filepath.Join(t.TempDir(), "nonexistent_procfile")
			}

			got, err := ParseProcfile(path)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseProcfile() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr {
				if got != nil {
					t.Errorf("ParseProcfile() got = %v, want nil on error", got)
				}
				if tt.errSubstr != "" && !strings.Contains(err.Error(), tt.errSubstr) {
					t.Errorf("ParseProcfile() error = %q, want substring %q", err.Error(), tt.errSubstr)
				}
				return
			}

			if len(got) != len(tt.want) {
				t.Fatalf("ParseProcfile() returned %d entries, want %d", len(got), len(tt.want))
			}

			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("ParseProcfile() entry[%d] = %+v, want %+v", i, got[i], tt.want[i])
				}
			}
		})
	}
}
