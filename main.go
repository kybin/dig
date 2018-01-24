package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"unicode/utf8"

	runewidth "github.com/mattn/go-runewidth"
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
		Win: &Window{
			Bound: main,
		},
	}
	return s
}

// Draw draws the screen.
func (s *Screen) Draw() {
	s.Side.Draw()
	s.Main.Draw()
}

// calcAreaBounds calculates it's sub area's boxes.
func calcAreaBounds(size Pt, hideSide bool) (side Rect, main Rect) {
	sideWidth := 40
	if size.O < 40 {
		sideWidth = size.O
	}
	if hideSide {
		sideWidth = 0
	}
	mainStart := sideWidth
	if !hideSide {
		mainStart += 3 // divide areas
	}
	mainWidth := size.O - mainStart
	if mainWidth < 0 {
		mainWidth = 0
	}
	side = Rect{
		Min:  Pt{0, 0},
		Size: Pt{size.L, sideWidth},
	}
	main = Rect{
		Min:  Pt{0, mainStart},
		Size: Pt{size.L, mainWidth},
	}
	return side, main
}

// Resize resizes the screen and re-fit sub areas.
func (s *Screen) Resize(size Pt) {
	s.size = size
	side, main := calcAreaBounds(size, s.hideSide)
	s.Side.Bound = side
	s.Main.Bound = main
}

// SideShowing returns whether Side area is showing or not.
func (s *Screen) SideShowing() bool {
	return !s.hideSide
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
	TopIdx  int
}

// Handle handles a terminal event.
func (a *ItemArea) Handle(ev termbox.Event) {
	switch ev.Key {
	case termbox.KeyArrowUp:
		a.CurIdx--
	case termbox.KeyArrowDown:
		a.CurIdx++
	case termbox.KeyPgup:
		a.CurIdx -= a.Bound.Size.L
	case termbox.KeyPgdn:
		a.CurIdx += a.Bound.Size.L
	case termbox.KeyHome:
		a.CurIdx = 0
	case termbox.KeyEnd:
		a.CurIdx = len(a.Commits) - 1
	}
	// validation
	if a.CurIdx < 0 {
		a.CurIdx = 0
	}
	if a.CurIdx >= len(a.Commits) {
		a.CurIdx = len(a.Commits) - 1
	}

	if a.TopIdx > a.CurIdx {
		a.TopIdx = a.CurIdx
	} else if a.TopIdx+a.Bound.Size.L <= a.CurIdx {
		a.TopIdx = a.CurIdx - a.Bound.Size.L + 1
	}
}

// Draw draws it's contents.
func (a *ItemArea) Draw() {
	top := a.TopIdx
	bottom := top + a.Bound.Size.L
	for i := top; i < bottom; i++ {
		if i == len(a.Commits) {
			break
		}
		commit := a.Commits[i]

		c := Color{Fg: termbox.ColorWhite, Bg: termbox.ColorBlack}
		if i == a.CurIdx {
			c = Color{Fg: termbox.ColorWhite, Bg: termbox.ColorGreen}
		}

		remain := commit.Title
		l := i - top
		o := 0
		for {
			if len(remain) == 0 {
				if i == a.CurIdx {
					// fill the rest of current line
					for o < a.Bound.Size.O {
						termbox.SetCell(a.Bound.Min.O+o, a.Bound.Min.L+l, ' ', c.Fg, c.Bg)
						o++
					}
				}
				break
			}
			if o >= a.Bound.Size.O {
				break
			}
			r, size := utf8.DecodeRuneInString(remain)
			remain = remain[size:]
			termbox.SetCell(o, l, r, c.Fg, c.Bg)
			o += runewidth.RuneWidth(r)
		}
	}
}

// Commit is currently selected commit.
func (a *ItemArea) Commit() *Commit {
	return a.Commits[a.CurIdx]
}

// DiffArea is an Area for showing diff outputs.
type DiffArea struct {
	CommitHash string
	Text       [][]byte

	Bound Rect
	Win   *Window
}

// Handle handles a terminal event.
func (a *DiffArea) Handle(ev termbox.Event) {
	if ev.Ch == 'f' || ev.Key == termbox.KeyPgdn {
		a.Win.PageForward()
	}
	if ev.Ch == 'b' || ev.Key == termbox.KeyPgup {
		a.Win.PageBackward()
	}
	if ev.Ch == 'd' {
		a.Win.HalfPageForward()
	}
	if ev.Ch == 'u' {
		a.Win.HalfPageBackward()
	}
	if ev.Ch == 'i' || ev.Key == termbox.KeyArrowUp {
		a.Win.MoveUp(8)
	}
	if ev.Ch == 'k' || ev.Key == termbox.KeyArrowDown {
		a.Win.MoveDown(8)
	}
	if ev.Ch == 'j' || ev.Key == termbox.KeyArrowLeft {
		a.Win.MoveLeft(16)
	}
	if ev.Ch == 'l' || ev.Key == termbox.KeyArrowRight {
		a.Win.MoveRight(16)
	}
}

// Draw draws it's contents.
func (a *DiffArea) Draw() {
	hash := screen.Side.Commit().Hash
	if hash != a.CommitHash {
		a.CommitHash = hash
		a.Text, _ = commitDiff(hash) // ignore error for now
		a.Win.Reset(a.Text)
	}
	minL := a.Win.Bound.Min.L
	maxL := a.Win.Bound.Min.L + a.Win.Bound.Size.L
	if maxL > len(a.Text) {
		maxL = len(a.Text)
	}
	for l, ln := range a.Text[minL:maxL] {
		c := Color{termbox.ColorWhite, termbox.ColorBlack}
		if len(ln) != 0 {
			first := string(ln[0])
			if first == "+" {
				c = Color{termbox.ColorGreen, termbox.ColorBlack}
			} else if first == "-" {
				c = Color{termbox.ColorRed, termbox.ColorBlack}
			}
		}
		o := 0
		remain := ln
		for {
			if len(remain) == 0 {
				break
			}
			if o >= a.Bound.Size.O {
				break
			}
			r, size := utf8.DecodeRune(remain)
			remain = remain[size:]
			termbox.SetCell(a.Bound.Min.O-a.Win.Bound.Min.O+o, a.Bound.Min.L+l, r, c.Fg, c.Bg)
			o += runewidth.RuneWidth(r)
		}
	}
}

// Window is a cursor which has size.
type Window struct {
	Bound Rect
	Text  [][]byte
}

func (w *Window) Reset(t [][]byte) {
	w.Text = t
	w.Bound.Min = Pt{0, 0}
}

// PageForward moves a window a page forward.
func (w *Window) PageForward() {
	w.MoveDown(w.Bound.Size.L)
}

// PageBackward moves a window a page backward.
func (w *Window) PageBackward() {
	w.MoveUp(w.Bound.Size.L)
}

// HalfPageForward moves a window a page forward.
func (w *Window) HalfPageForward() {
	w.MoveDown(w.Bound.Size.L / 2)
}

// HalfPageBackward moves a window a page backward.
func (w *Window) HalfPageBackward() {
	w.MoveUp(w.Bound.Size.L / 2)
}

// MoveUp moves up at maximum n.
// When it hits the boundary it stops.
func (w *Window) MoveUp(n int) {
	w.Bound.Min.L -= n
	if w.Bound.Min.L < 0 {
		w.Bound.Min.L = 0
	}
}

// MoveDown moves down at maximum n.
// When it hits the boundary it stops.
func (w *Window) MoveDown(n int) {
	w.Bound.Min.L += n
	if w.Bound.Min.L >= len(w.Text) {
		w.Bound.Min.L = len(w.Text) - 1
	}
}

// MoveLeft moves left at maximum n.
// When it hits the boundary it stops.
func (w *Window) MoveLeft(n int) {
	w.Bound.Min.O -= n
	if w.Bound.Min.O < 0 {
		w.Bound.Min.O = 0
	}
}

// MoveRight move right at maximum n.
// TODO: When it hits the boundary it stops.
func (w *Window) MoveRight(n int) {
	w.Bound.Min.O += n
}

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
	Hash  string
	Title string
}

// allCommits find a repository and get it's commits.
func allCommits(repodir string) ([]*Commit, error) {
	cmd := exec.Command("git", "log", "--pretty=format:%H%n%s%n")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, err
	}
	// tab handling in screen is quite awkard. handle it here.
	out = bytes.Replace(out, []byte("\t"), []byte("    "), -1)
	commits := []*Commit{}
	commitStrings := strings.Split(string(out), "\n\n")
	last := len(commitStrings) - 1
	for i := range commitStrings {
		c := commitStrings[last-i] // first commit live at last.
		l := strings.Split(c, "\n")
		commits = append(commits, &Commit{Hash: l[0], Title: l[1]})
	}
	return commits, nil
}

// commitDiff returns changes of a commit.
func commitDiff(hash string) ([][]byte, error) {
	cmd := exec.Command("git", "show", hash)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, err
	}
	// tab handling in screen is quite awkard. handle it here.
	out = bytes.Replace(out, []byte("\t"), []byte("    "), -1)
	lines := bytes.Split(out, []byte("\n"))
	return lines, err
}

func main() {
	commits, err := allCommits(".")
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
	screen = NewScreen(size, commits)

	events := make(chan termbox.Event, 20)
	go func() {
		for {
			events <- termbox.PollEvent()
		}
	}()

	curArea := Area(screen.Side)
	for {
		termbox.Clear(termbox.ColorDefault, termbox.ColorDefault)
		screen.Draw()
		termbox.Flush()

		ev := <-events
		switch ev.Type {
		case termbox.EventKey:
			// handle global event
			switch ev.Key {
			case termbox.KeyCtrlQ:
				return
			case termbox.KeyEnter:
				// toggle side
				if screen.SideShowing() {
					screen.ShowSide(false)
					curArea = screen.Main
				} else {
					screen.ShowSide(true)
					curArea = screen.Side
				}
			case termbox.KeyEsc:
				screen.ShowSide(true)
				curArea = screen.Side
			}
			// handle sub area event
			curArea.Handle(ev)
		case termbox.EventResize:
			w, h := termbox.Size()
			size := Pt{h, w}
			screen.Resize(size)
		}
	}
}
