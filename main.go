package main

import termbox "github.com/nsf/termbox-go"

type Screen struct {
	Side *ItemArea
	Main *DiffArea

	ShowSide bool
}

// Draw
// Resize

type Area interface {
	Handle(termbox.Event) (next Area)
	Draw()
	ResetBox(Rect)
}

type ItemArea struct {
	box     Rect
	Commits []*Commit
	CurIdx  int
}

// Handle
// Draw
// ResetBox

type DiffArea struct {
	box  Rect
	Text []byte
	Win  *Window
}

// Handle
// Draw
// ResetBox

type Window struct {
	Cursor // Window is Cursor
	Size   Pt
}

type Cursor struct {
	Pos  Pt
	Text []byte
}

// PageForward
// PageBackward
// HalfPageForward
// HalfPageBackward
// MoveUp
// MoveDown
// MoveLeft
// MoveRight

type Rect struct {
	Min  Pt
	Size Pt
}

type Pt struct {
	L int
	O int
}

// Move

type Commit struct {
	Hash string
	Diff []byte
}

var screen *Screen

func main() {
	commits, err := AllCommits(repoDir)
	screen, err := NewScreen(size, commits)
	for ev {
		// handle global events
		curArea = curArea.Handle(ev)
		if curArea == nil {
			return
		}
		screen.Draw()
	}
}
