package validor

import "testing"

func TestBoolToStr(t *testing.T) {
	tests := []struct {
		name string
		cond bool
		yes  string
		no   string
		want string
	}{
		{
			name: "condition true",
			cond: true,
			yes:  "YES",
			no:   "NO",
			want: "YES",
		},
		{
			name: "condition false",
			cond: false,
			yes:  "YES",
			no:   "NO",
			want: "NO",
		},
		{
			name: "empty strings",
			cond: true,
			yes:  "",
			no:   "something",
			want: "",
		},
		{
			name: "complex strings",
			cond: false,
			yes:  "This is the true value",
			no:   "This is the false value",
			want: "This is the false value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BoolToStr(tt.cond, tt.yes, tt.no)
			if got != tt.want {
				t.Errorf("BoolToStr() = %v, want %v", got, tt.want)
			}
		})
	}
}
