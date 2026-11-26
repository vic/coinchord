package layer

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// Button is a mouse button.
type Button uint8

const (
	LeftButton Button = iota
	RightButton
	MiddleButton
)

var buttonNames = []string{"left", "right", "middle"}

func (b Button) String() string { return buttonNames[b] }

// MouseKind says what a mouse action does.
type MouseKind uint8

const (
	Move   MouseKind = iota // move the pointer by DX, DY pixels
	Scroll                  // scroll by DX, DY lines (DY > 0 = down)
	Click                   // press and release Button
	Double                  // two clicks
	Down                    // press and hold Button (start a drag)
	Up                      // release Button (end a drag)
)

var mouseNames = []string{"move", "scroll", "click", "double", "down", "up"}

// Mouse is a pointer action. Build it with parseMouse.
type Mouse struct {
	Kind   MouseKind
	Button Button
	DX, DY int
}

func (Mouse) act() {}

func (m Mouse) String() string {
	switch m.Kind {
	case Move, Scroll:
		return fmt.Sprintf("%s:%d,%d", mouseNames[m.Kind], m.DX, m.DY)
	}
	return mouseNames[m.Kind] + ":" + m.Button.String()
}

const maxStep = 2000 // larger moves or scrolls are typos

func parseMouse(kind, arg string) (Mouse, error) {
	k := MouseKind(slices.Index(mouseNames, kind))
	switch k {
	case Move, Scroll:
		x, y, two := strings.Cut(arg, ",")
		switch {
		case !two && k == Scroll: // {scroll:3} scrolls vertically
			x, y = "0", x
		case !two:
			return Mouse{}, fmt.Errorf("{move:%s}: want DX,DY", arg)
		}
		dx, err1 := strconv.Atoi(strings.TrimSpace(x))
		dy, err2 := strconv.Atoi(strings.TrimSpace(y))
		if err1 != nil || err2 != nil || abs(dx) > maxStep || abs(dy) > maxStep {
			return Mouse{}, fmt.Errorf("{%s:%s}: want whole numbers DX,DY up to %d", kind, arg, maxStep)
		}
		return Mouse{Kind: k, DX: dx, DY: dy}, nil
	}
	b := slices.Index(buttonNames, arg)
	if b < 0 {
		return Mouse{}, fmt.Errorf("{%s:%s}: button must be left, right or middle", kind, arg)
	}
	return Mouse{Kind: k, Button: Button(b)}, nil
}

func abs(x int) int { return max(x, -x) }

// Command is a portable host action such as "next window". Each system
// gets its own shortcut for it.
type Command uint8

const (
	CmdSwitch Command = iota
	CmdSwitchBack
	CmdClose
	CmdMaximize
	CmdMinimize
	CmdSnapLeft
	CmdSnapRight
	CmdDesktopNext
	CmdDesktopPrev
	CmdShowDesktop
	CmdLauncher
	CmdFullscreen
	CmdTabNext
	CmdTabPrev
	CmdTabNew
	CmdTabClose
	CmdBack
	CmdForward
	CmdCopy
	CmdPaste
	CmdCut
	CmdUndo
	CmdRedo
	CmdSave
	CmdFind
	CmdSelectAll
	CmdWordLeft
	CmdWordRight
	CmdDocStart
	CmdDocEnd
	commandCount
)

var commandNames = [commandCount]string{
	"switch", "switch-back", "close", "maximize", "minimize", "snap-left", "snap-right",
	"desktop-next", "desktop-prev", "show-desktop", "launcher", "fullscreen",
	"tab-next", "tab-prev", "tab-new", "tab-close", "back", "forward",
	"copy", "paste", "cut", "undo", "redo", "save", "find", "select-all",
	"word-left", "word-right", "doc-start", "doc-end",
}

var commandWords = [commandCount]string{
	"switch to the next window", "switch to the previous window", "close the window", "maximize the window",
	"minimize the window", "snap the window left", "snap the window right",
	"go to the next desktop", "go to the previous desktop", "show the desktop", "open the app launcher", "toggle full screen",
	"next tab", "previous tab", "new tab", "close the tab", "go back", "go forward",
	"copy", "paste", "cut", "undo", "redo", "save", "find", "select all",
	"jump one word left", "jump one word right", "go to the start", "go to the end",
}

func (Command) act() {}

func (c Command) String() string { return commandNames[c] }

// Words describes the command for people.
func (c Command) Words() string { return commandWords[c] }

// Commands lists every command.
func Commands() []Command {
	out := make([]Command, commandCount)
	for i := range out {
		out[i] = Command(i)
	}
	return out
}

func parseCommand(name string) (Command, error) {
	i := slices.Index(commandNames[:], name)
	if i < 0 {
		return 0, fmt.Errorf("{do:%s}: unknown command; known: %s", name, strings.Join(commandNames[:], " "))
	}
	return Command(i), nil
}

// shortcuts per system. Missing entries mean the system has no shortcut.
var shortcuts = map[string]map[Command]string{
	"linux": {
		CmdSwitch: "alt+tab", CmdSwitchBack: "alt+shift+tab", CmdClose: "alt+f4", CmdMaximize: "super+up",
		CmdMinimize: "super+h", CmdSnapLeft: "super+left", CmdSnapRight: "super+right",
		CmdDesktopNext: "super+pgdn", CmdDesktopPrev: "super+pgup", CmdShowDesktop: "super+d",
		CmdLauncher: "super+a", CmdFullscreen: "f11",
	},
	"windows": {
		CmdSwitch: "alt+tab", CmdSwitchBack: "alt+shift+tab", CmdClose: "alt+f4", CmdMaximize: "super+up",
		CmdMinimize: "super+down", CmdSnapLeft: "super+left", CmdSnapRight: "super+right",
		CmdDesktopNext: "ctrl+super+right", CmdDesktopPrev: "ctrl+super+left", CmdShowDesktop: "super+d",
		CmdLauncher: "super+s", CmdFullscreen: "f11",
	},
	"darwin": {
		CmdSwitch: "super+tab", CmdSwitchBack: "super+shift+tab", CmdClose: "super+w", CmdMaximize: "ctrl+super+f",
		CmdMinimize: "super+m", CmdDesktopNext: "ctrl+right", CmdDesktopPrev: "ctrl+left",
		CmdShowDesktop: "f11", CmdLauncher: "super+space", CmdFullscreen: "ctrl+super+f",
		CmdTabNext: "ctrl+tab", CmdTabPrev: "ctrl+shift+tab", CmdTabNew: "super+t", CmdTabClose: "super+w",
		CmdBack: "super+[", CmdForward: "super+]", CmdCopy: "super+c", CmdPaste: "super+v", CmdCut: "super+x",
		CmdUndo: "super+z", CmdRedo: "super+shift+z", CmdSave: "super+s", CmdFind: "super+f", CmdSelectAll: "super+a",
		CmdWordLeft: "alt+left", CmdWordRight: "alt+right", CmdDocStart: "super+up", CmdDocEnd: "super+down",
	},
}

// pcShortcuts are shared by linux and windows.
var pcShortcuts = map[Command]string{
	CmdTabNext: "ctrl+tab", CmdTabPrev: "ctrl+shift+tab", CmdTabNew: "ctrl+t", CmdTabClose: "ctrl+w",
	CmdBack: "alt+left", CmdForward: "alt+right", CmdCopy: "ctrl+c", CmdPaste: "ctrl+v", CmdCut: "ctrl+x",
	CmdUndo: "ctrl+z", CmdRedo: "ctrl+shift+z", CmdSave: "ctrl+s", CmdFind: "ctrl+f", CmdSelectAll: "ctrl+a",
	CmdWordLeft: "ctrl+left", CmdWordRight: "ctrl+right", CmdDocStart: "ctrl+home", CmdDocEnd: "ctrl+end",
}

// Shortcut returns the key combo of c on system goos (a GOOS value).
func (c Command) Shortcut(goos string) (Combo, error) {
	s, ok := shortcuts[goos][c]
	if !ok && goos != "darwin" {
		s, ok = pcShortcuts[c]
	}
	if !ok {
		return Combo{}, fmt.Errorf("%s has no shortcut on %s", c, goos)
	}
	return parseCombo(s)
}
