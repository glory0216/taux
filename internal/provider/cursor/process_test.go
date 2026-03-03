package cursor

import "testing"

func TestIsCursorProcess_MacMain(t *testing.T) {
	line := "12345  51200  1.5 /Applications/Cursor.app/Contents/MacOS/Cursor --type=main"
	if !isCursorProcess(line) {
		t.Error("expected true for Cursor.app/Contents/MacOS/Cursor")
	}
}

func TestIsCursorProcess_LinuxCursor(t *testing.T) {
	line := "67890  32000  0.5 /usr/bin/cursor --flag"
	if !isCursorProcess(line) {
		t.Error("expected true for /usr/bin/cursor --flag")
	}
}

func TestIsCursorProcess_BareTrailing(t *testing.T) {
	line := "11111  20000  0.2 /path/to/cursor"
	if !isCursorProcess(line) {
		t.Error("expected true for path ending with /cursor (HasSuffix)")
	}
}

func TestIsCursorProcess_Helper(t *testing.T) {
	line := "22222  15000  0.3 /Applications/Cursor.app/Contents/Frameworks/Cursor Helper (GPU).app/Contents/MacOS/Cursor Helper (GPU)"
	if isCursorProcess(line) {
		t.Error("expected false for Cursor Helper (GPU)")
	}
}

func TestIsCursorProcess_CursorHelper(t *testing.T) {
	line := "33333  10000  0.1 /usr/local/bin/cursor-helper --worker"
	if isCursorProcess(line) {
		t.Error("expected false for cursor-helper")
	}
}

func TestIsCursorProcess_Crashpad(t *testing.T) {
	line := "44444  5000  0.0 /Applications/Cursor.app/Contents/Frameworks/Cursor Helper.app/Contents/MacOS/crashpad_handler --monitor-self"
	if isCursorProcess(line) {
		t.Error("expected false for crashpad_handler")
	}
}

func TestIsCursorProcess_Unrelated(t *testing.T) {
	line := "55555  80000  2.1 node /home/user/project/server.js"
	if isCursorProcess(line) {
		t.Error("expected false for unrelated process")
	}
}
