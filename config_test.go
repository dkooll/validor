package validor

import (
	"reflect"
	"testing"
)

func TestConfig_ParseExceptionList(t *testing.T) {
	tests := []struct {
		name      string
		exception string
		want      []string
	}{
		{
			name:      "empty exception",
			exception: "",
			want:      []string{},
		},
		{
			name:      "single exception",
			exception: "example1",
			want:      []string{"example1"},
		},
		{
			name:      "multiple exceptions",
			exception: "example1,example2,example3",
			want:      []string{"example1", "example2", "example3"},
		},
		{
			name:      "exceptions with spaces",
			exception: " example1 , example2 , example3 ",
			want:      []string{"example1", "example2", "example3"},
		},
		{
			name:      "exceptions with trailing comma",
			exception: "example1,example2,",
			want:      []string{"example1", "example2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Config{Exception: tt.exception}
			c.ParseExceptionList()
			if !reflect.DeepEqual(c.ExceptionList, tt.want) {
				t.Errorf("ParseExceptionList() = %v, want %v", c.ExceptionList, tt.want)
			}
		})
	}
}

func TestNewConfig(t *testing.T) {
	tests := []struct {
		name string
		opts []Option
		want *Config
	}{
		{
			name: "default config",
			opts: []Option{},
			want: &Config{
				SkipDestroy:   false,
				Exception:     "",
				Example:       "",
				Local:         false,
				ExceptionList: nil,
				Namespace:     "",
				ExamplesPath:  "",
			},
		},
		{
			name: "config with skip destroy",
			opts: []Option{WithSkipDestroy(true)},
			want: &Config{
				SkipDestroy: true,
			},
		},
		{
			name: "config with exception",
			opts: []Option{WithException("example1,example2")},
			want: &Config{
				Exception:     "example1,example2",
				ExceptionList: []string{"example1", "example2"},
			},
		},
		{
			name: "config with example",
			opts: []Option{WithExample("example1")},
			want: &Config{
				Example: "example1",
			},
		},
		{
			name: "config with local",
			opts: []Option{WithLocal(true)},
			want: &Config{
				Local: true,
			},
		},
		{
			name: "config with examples path",
			opts: []Option{WithExamplesPath("/custom/path")},
			want: &Config{
				ExamplesPath: "/custom/path",
			},
		},
		{
			name: "config with multiple options",
			opts: []Option{
				WithSkipDestroy(true),
				WithLocal(true),
				WithExample("test1"),
				WithExamplesPath("/path"),
			},
			want: &Config{
				SkipDestroy:  true,
				Local:        true,
				Example:      "test1",
				ExamplesPath: "/path",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewConfig(tt.opts...)

			if got.SkipDestroy != tt.want.SkipDestroy {
				t.Errorf("SkipDestroy = %v, want %v", got.SkipDestroy, tt.want.SkipDestroy)
			}
			if got.Local != tt.want.Local {
				t.Errorf("Local = %v, want %v", got.Local, tt.want.Local)
			}
			if got.Example != tt.want.Example {
				t.Errorf("Example = %v, want %v", got.Example, tt.want.Example)
			}
			if got.ExamplesPath != tt.want.ExamplesPath {
				t.Errorf("ExamplesPath = %v, want %v", got.ExamplesPath, tt.want.ExamplesPath)
			}
			if tt.want.ExceptionList != nil && !reflect.DeepEqual(got.ExceptionList, tt.want.ExceptionList) {
				t.Errorf("ExceptionList = %v, want %v", got.ExceptionList, tt.want.ExceptionList)
			}
		})
	}
}

func TestWithOptions(t *testing.T) {
	t.Run("WithSkipDestroy", func(t *testing.T) {
		c := &Config{}
		WithSkipDestroy(true)(c)
		if !c.SkipDestroy {
			t.Errorf("WithSkipDestroy(true) did not set SkipDestroy to true")
		}
	})

	t.Run("WithException", func(t *testing.T) {
		c := &Config{}
		WithException("ex1,ex2")(c)
		if c.Exception != "ex1,ex2" {
			t.Errorf("WithException did not set Exception correctly")
		}
		if !reflect.DeepEqual(c.ExceptionList, []string{"ex1", "ex2"}) {
			t.Errorf("WithException did not parse ExceptionList correctly: got %v", c.ExceptionList)
		}
	})

	t.Run("WithExample", func(t *testing.T) {
		c := &Config{}
		WithExample("test")(c)
		if c.Example != "test" {
			t.Errorf("WithExample did not set Example correctly")
		}
	})

	t.Run("WithLocal", func(t *testing.T) {
		c := &Config{}
		WithLocal(true)(c)
		if !c.Local {
			t.Errorf("WithLocal(true) did not set Local to true")
		}
	})

	t.Run("WithExamplesPath", func(t *testing.T) {
		c := &Config{}
		WithExamplesPath("/test/path")(c)
		if c.ExamplesPath != "/test/path" {
			t.Errorf("WithExamplesPath did not set ExamplesPath correctly")
		}
	})
}

func TestGetExamplesPath(t *testing.T) {
	tests := []struct {
		name   string
		config *Config
		want   string
	}{
		{
			name:   "custom path set",
			config: &Config{ExamplesPath: "/custom/examples"},
			want:   "/custom/examples",
		},
		{
			name:   "default path",
			config: &Config{},
			want:   "../examples",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getExamplesPath(tt.config)
			if got != tt.want {
				t.Errorf("getExamplesPath() = %v, want %v", got, tt.want)
			}
		})
	}
}
