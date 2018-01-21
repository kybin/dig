package main

import (
	"fmt"
	"os"

	termbox "github.com/nsf/termbox-go"
)

// screen is a singleton variable for Screen.
var screen *Screen

// Screen is a program screen.
type Screen struct {
	size     Pt
	hideSide bool

	Side *ItemArea
	Main *DiffArea
}

// NewScreen creates a new Screen.
// It will also create it's sub areas.
func NewScreen(size Pt, commits []*Commit) *Screen {
	side, main := calcAreaBounds(size, false)
	s := &Screen{size: size}
	s.Side = &ItemArea{
		Bound:   side,
		Commits: commits,
	}
	s.Main = &DiffArea{
		Bound: main,
	}
	return s
}

// Draw draw the screen content.
func (s *Screen) Draw() {
	s.Side.Draw()
	s.Main.Draw()
}

// calcAreaBounds calculates it's sub area's boxes.
func calcAreaBounds(size Pt, hideSide bool) (side Rect, main Rect) {
	sideWidth := 20
	if size.L < 20 {
		sideWidth = size.O
	}
	if hideSide {
		sideWidth = 0
	}
	mainWidth := size.O - sideWidth
	side = Rect{
		Min:  Pt{0, 0},
		Size: Pt{size.L, sideWidth},
	}
	main = Rect{
		Min:  Pt{0, sideWidth},
		Size: Pt{size.L, mainWidth},
	}
	return side, main
}

// Resize resize the screen and re-fit sub areas.
func (s *Screen) Resize(size Pt) {
	side, main := calcAreaBounds(size, s.hideSide)
	s.Side.Bound = side
	s.Main.Bound = main
}

// ShowSide shows or hides it's Side screen.
func (s *Screen) ShowSide(show bool) {
	s.hideSide = !show
	s.Resize(s.size)
}

// Area handles terminal events and draw it's contents.
type Area interface {
	Handle(termbox.Event)
	Draw()
}

// fillColor fills color to the bound.
func fillColor(bound Rect, c Color) {
	min := bound.Min
	max := bound.Min.Add(bound.Size)
	for l := min.L; l < max.L; l++ {
		for o := min.O; o < max.O; o++ {
			termbox.SetCell(o, l, 's', c.Fg, c.Bg)
		}
	}
}

// ItemArea is an Area for showing commits.
type ItemArea struct {
	Bound   Rect
	Commits []*Commit
	CurIdx  int
}

// Handle handles a terminal event.
func (a *ItemArea) Handle(ev termbox.Event) {}

// Draw draws it's contents.
func (a *ItemArea) Draw() {
	fillColor(a.Bound, Color{termbox.ColorRed, termbox.ColorRed})
}

// DiffArea is an Area for showing diff outputs.
type DiffArea struct {
	Bound Rect
	Text  []byte
	Win   *Window
}

// Handle handles a terminal event.
func (a *DiffArea) Handle(termbox.Event) {}

// Draw draws it's contents.
func (a *DiffArea) Draw() {
	fillColor(a.Bound, Color{termbox.ColorBlue, termbox.ColorBlue})
}

// Window is a cursor which has size.
type Window struct {
	Cursor
	Size Pt
}

// Cursor is a cursor to navigate it's text.
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

// Rect is a rectangle.
type Rect struct {
	Min  Pt
	Size Pt
}

// Pt is a point.
type Pt struct {
	L int
	O int
}

// Add adds two points and returns the result.
func (p Pt) Add(q Pt) Pt {
	return Pt{p.L + q.L, p.O + q.O}
}

// Color is terminal color.
type Color struct {
	Fg termbox.Attribute
	Bg termbox.Attribute
}

// Commit is a git commit.
type Commit struct {
	Hash string
	Diff []byte
}

// allCommits find repository and get it's commits.
func allCommits(repodir string) ([]*Commit, error) {
	return []*Commit{}, nil
}

func main() {
	commits, err := allCommits("implement this")
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not get commits: %v", err)
	}

	err = termbox.Init()
	if err != nil {
		fmt.Fprint(os.Stderr, err)
		os.Exit(1)
	}
	defer termbox.Close()

	w, h := termbox.Size()
	size := Pt{h, w}
	screen := NewScreen(size, commits)

	events := make(chan termbox.Event, 20)
	go func() {
		for {
			events <- termbox.PollEvent()
		}
	}()

	curArea := Area(screen.Side)
	for {
		screen.Draw()
		termbox.Flush()

		ev := <-events
		switch ev.Type {
		case termbox.EventKey:
			// handle global event
			switch ev.Key {
			case termbox.KeyCtrlW:
				return
			case termbox.KeyEnter:
				screen.ShowSide(false)
				curArea = screen.Main
			case termbox.KeyEsc:
				screen.ShowSide(true)
				curArea = screen.Side
			case termbox.KeyArrowRight:
				curArea = screen.Main
			case termbox.KeyArrowLeft:
				curArea = screen.Side
			}
			// handle sub area event
			curArea.Handle(ev)
		}
	}
}
