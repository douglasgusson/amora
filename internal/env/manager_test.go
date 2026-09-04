package env

import (
	"os"
	"path/filepath"
	"testing"
)

func TestManagerLoad(t *testing.T) {
	tests := []struct {
		name       string
		createFile bool
		content    string
		want       map[string]string
		wantErr    bool
	}{
		{
			name:       "nonexistent_file",
			createFile: false,
			want:       map[string]string{},
			wantErr:    false,
		},
		{
			name:       "valid_env_file",
			createFile: true,
			content:    "PORT=5000\nNODE_ENV=production\n",
			want: map[string]string{
				"PORT":     "5000",
				"NODE_ENV": "production",
			},
			wantErr: false,
		},
		{
			name:       "comments_and_blanks",
			createFile: true,
			content:    "# This is a comment\n\nKEY=value\n   \n# Another comment\n",
			want: map[string]string{
				"KEY": "value",
			},
			wantErr: false,
		},
		{
			name:       "line_without_equals",
			createFile: true,
			content:    "INVALID_LINE\nVALID=true\nANOTHER_INVALID\n",
			want: map[string]string{
				"VALID": "true",
			},
			wantErr: false,
		},
		{
			name:       "value_with_equals",
			createFile: true,
			content:    "DATABASE_URL=postgres://user:pass@host/db?opt=val\n",
			want: map[string]string{
				"DATABASE_URL": "postgres://user:pass@host/db?opt=val",
			},
			wantErr: false,
		},
		{
			name:       "whitespace_trimming",
			createFile: true,
			content:    "  KEY  =  VALUE  \n",
			want: map[string]string{
				"KEY": "VALUE",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			mgr := NewManager(tmpDir)
			app := "myapp"

			if tt.createFile {
				path := mgr.FilePath(app)
				if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
					t.Fatalf("failed to create directory: %v", err)
				}
				if err := os.WriteFile(path, []byte(tt.content), 0644); err != nil {
					t.Fatalf("failed to seed env file: %v", err)
				}
			}

			got, err := mgr.Load(app)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Load() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got == nil {
				t.Fatal("Load() returned nil map, want initialized empty map")
			}
			if len(got) != len(tt.want) {
				t.Fatalf("Load() returned %d entries, want %d: got=%v, want=%v", len(got), len(tt.want), got, tt.want)
			}
			for k, wantVal := range tt.want {
				gotVal, ok := got[k]
				if !ok {
					t.Errorf("Load() missing key %q", k)
				} else if gotVal != wantVal {
					t.Errorf("Load()[%q] = %q, want %q", k, gotVal, wantVal)
				}
			}
		})
	}
}

func TestManagerSave(t *testing.T) {
	t.Run("save_new_file", func(t *testing.T) {
		tmpDir := t.TempDir()
		mgr := NewManager(tmpDir)
		app := "newapp"

		vars := map[string]string{
			"PORT": "5000",
			"HOST": "localhost",
		}

		if err := mgr.Save(app, vars); err != nil {
			t.Fatalf("Save() error = %v", err)
		}

		data, err := os.ReadFile(mgr.FilePath(app))
		if err != nil {
			t.Fatalf("failed to read saved env file: %v", err)
		}

		want := "HOST=localhost\nPORT=5000\n"
		if string(data) != want {
			t.Errorf("file content = %q, want %q", string(data), want)
		}
	})

	t.Run("save_overwrites", func(t *testing.T) {
		tmpDir := t.TempDir()
		mgr := NewManager(tmpDir)
		app := "myapp"

		initialVars := map[string]string{
			"OLD_KEY": "old_value",
			"EXTRA":   "extra_value",
		}
		if err := mgr.Save(app, initialVars); err != nil {
			t.Fatalf("first Save() error = %v", err)
		}

		newVars := map[string]string{
			"NEW_KEY": "new_value",
		}
		if err := mgr.Save(app, newVars); err != nil {
			t.Fatalf("second Save() error = %v", err)
		}

		data, err := os.ReadFile(mgr.FilePath(app))
		if err != nil {
			t.Fatalf("failed to read saved env file: %v", err)
		}

		want := "NEW_KEY=new_value\n"
		if string(data) != want {
			t.Errorf("file content = %q, want %q", string(data), want)
		}
	})

	t.Run("save_sorted_keys", func(t *testing.T) {
		tmpDir := t.TempDir()
		mgr := NewManager(tmpDir)
		app := "myapp"

		vars := map[string]string{
			"Z": "zebra",
			"A": "apple",
			"M": "mango",
		}
		if err := mgr.Save(app, vars); err != nil {
			t.Fatalf("Save() error = %v", err)
		}

		data, err := os.ReadFile(mgr.FilePath(app))
		if err != nil {
			t.Fatalf("failed to read saved env file: %v", err)
		}

		want := "A=apple\nM=mango\nZ=zebra\n"
		if string(data) != want {
			t.Errorf("file content = %q, want %q", string(data), want)
		}
	})
}

func TestManagerSet(t *testing.T) {
	t.Run("set_creates_and_updates", func(t *testing.T) {
		tmpDir := t.TempDir()
		mgr := NewManager(tmpDir)
		app := "myapp"

		// Set on empty app
		if err := mgr.Set(app, "PORT", "5000"); err != nil {
			t.Fatalf("first Set() error = %v", err)
		}

		data, err := os.ReadFile(mgr.FilePath(app))
		if err != nil {
			t.Fatalf("failed to read env file after first Set: %v", err)
		}
		if string(data) != "PORT=5000\n" {
			t.Errorf("file content after first Set = %q, want %q", string(data), "PORT=5000\n")
		}

		// Set again on same app -> both values present
		if err := mgr.Set(app, "NODE_ENV", "production"); err != nil {
			t.Fatalf("second Set() error = %v", err)
		}

		data, err = os.ReadFile(mgr.FilePath(app))
		if err != nil {
			t.Fatalf("failed to read env file after second Set: %v", err)
		}
		want := "NODE_ENV=production\nPORT=5000\n"
		if string(data) != want {
			t.Errorf("file content after second Set = %q, want %q", string(data), want)
		}
	})
}
