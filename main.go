package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"unicode/utf8"

	runewidth "github.com/mattn/go-runewidth"
	termbox "github.com/nsf/termbox-go"
)

// dig indicates this program.
var dig *Program

// Program is a program.
type Program struct {
	Mode    Mode
	RepoDir string
	Commits []*Commit

	FindString string
}

// Mode is mode of program.
type Mode int

const (
	NormalMode = Mode(iota)
	FindMode
)

// screen indicates this program screen.
var screen *Screen

// Screen is a program screen.
type Screen struct {
	size     Pt
	hideSide bool

	visibleSideWidth   int
	invisibleSideWidth int

	Side   *ItemArea
	Main   *DiffArea
	Status *StatusArea
}

// NewScreen creates a new Screen.
// It will also create it's sub areas.
func NewScreen(size Pt) *Screen {
	s := &Screen{
		size:               size,
		visibleSideWidth:   60,
		invisibleSideWidth: 30,
		Side:               &ItemArea{},
		Main:               &DiffArea{Win: &Window{}},
		Status:             &StatusArea{},
	}
	s.Resize(size)
	return s
}

// Draw draws the screen.
func (s *Screen) Draw() {
	if !s.hideSide {
		s.Side.Draw()
	}
	s.Main.Draw()
	s.Status.Draw()
}

// Resize resizes the screen and re-fit sub areas.
func (s *Screen) Resize(size Pt) {
	s.size = size

	sideWidth := s.visibleSideWidth
	if s.hideSide {
		sideWidth = s.invisibleSideWidth
	}
	if sideWidth > size.O {
		sideWidth = size.O
	}

	mainStart := sideWidth
	if !s.hideSide {
		mainStart += 3 // divide areas
	}
	mainWidth := size.O - mainStart
	if mainWidth < 0 {
		mainWidth = 0
	}

	s.Side.Bound = Rect{
		Min:  Pt{0, 0},
		Size: Pt{size.L - 1, sideWidth},
	}
	s.Main.Bound = Rect{
		Min:  Pt{0, mainStart},
		Size: Pt{size.L - 1, mainWidth},
	}
	s.Main.Win.Bound.Size = s.Main.Bound.Size
	s.Status.Bound = Rect{
		Min:  Pt{size.L - 1, 0},
		Size: Pt{1, size.O},
	}
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

// ExpandSide expands or shirinks it's Side screen.
func (s *Screen) ExpandSide(n int) {
	if s.hideSide {
		s.invisibleSideWidth += n
		if s.invisibleSideWidth < 0 {
			s.invisibleSideWidth = 0
		}
	} else {
		s.visibleSideWidth += n
		if s.visibleSideWidth < 0 {
			s.visibleSideWidth = 0
		}
	}
	s.Resize(s.size)
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
		a.CurIdx = len(dig.Commits) - 1
	}
	// validation
	if a.CurIdx < 0 {
		a.CurIdx = 0
	}
	if a.CurIdx >= len(dig.Commits) {
		a.CurIdx = len(dig.Commits) - 1
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
		if i == len(dig.Commits) {
			break
		}
		commit := dig.Commits[i]

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
	return dig.Commits[a.CurIdx]
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
	if ev.Ch == 'f' {
		a.Win.PageForward()
	}
	if ev.Ch == 'b' {
		a.Win.PageBackward()
	}
	if ev.Ch == 'd' {
		a.Win.HalfPageForward()
	}
	if ev.Ch == 'u' {
		a.Win.HalfPageBackward()
	}
	if ev.Ch == 'i' {
		a.Win.MoveUp(1)
	}
	if ev.Ch == 'k' {
		a.Win.MoveDown(1)
	}
	if ev.Ch == 'j' {
		a.Win.MoveLeft(4)
	}
	if ev.Ch == 'l' {
		a.Win.MoveRight(4)
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
		// relative offset in window
		// we can't just clipping remain, as we did with a.Text's lines (l).
		// because o should be calculated rune by rune.
		o := -a.Win.Bound.Min.O
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
			if o >= 0 {
				termbox.SetCell(a.Bound.Min.O+o, a.Bound.Min.L+l, r, c.Fg, c.Bg)
			}
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

type StatusArea struct {
	Bound Rect
}

func (a StatusArea) Draw() {
	var drawString string
	if dig.Mode == NormalMode {
		drawString = "q: quit, Down: next commit, Up: prev commit, f: page down, b: page up, <: shirink side, >: expand side"
	} else if dig.Mode == FindMode {
		drawString = "find: " + dig.FindString
	}
	remain := drawString
	o := 0
	for {
		if len(remain) == 0 {
			break
		}
		r, size := utf8.DecodeRuneInString(remain)
		remain = remain[size:]
		termbox.SetCell(o, a.Bound.Min.L, r, termbox.ColorBlack, termbox.ColorWhite)
		o += runewidth.RuneWidth(r)
	}
	for o < a.Bound.Size.O {
		termbox.SetCell(o, a.Bound.Min.L, ' ', termbox.ColorBlack, termbox.ColorWhite)
		o++
	}
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
func allCommits(repodir string, digUp bool) ([]*Commit, error) {
	cmd := exec.Command("git", "log", "--pretty=format:%H%n%s%n")
	cmd.Dir = repodir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, errors.New(string(out))
	}
	// tab handling in screen is quite awkard. handle it here.
	out = bytes.Replace(out, []byte("\t"), []byte("    "), -1)
	commits := []*Commit{}
	commitStrings := strings.Split(string(out), "\n\n")
	last := len(commitStrings) - 1
	for i := range commitStrings {
		j := i
		if digUp {
			j = last - i
		}
		c := commitStrings[j] // first commit live at last.
		l := strings.Split(c, "\n")
		commits = append(commits, &Commit{Hash: l[0], Title: l[1]})
	}
	return commits, nil
}

// commitDiff returns changes of a commit.
func commitDiff(hash string) ([][]byte, error) {
	cmd := exec.Command("git", "show", hash)
	cmd.Dir = dig.RepoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, err
	}
	// tab handling in screen is quite awkard. handle it here.
	out = bytes.Replace(out, []byte("\t"), []byte("    "), -1)
	lines := bytes.Split(out, []byte("\n"))
	return lines, err
}

// handleNormal handles NormalMode events.
// When the event was handled, it will return true.
func handleNormal(ev termbox.Event) bool {
	switch ev.Key {
	case termbox.KeyEnter:
		// toggle side
		if screen.SideShowing() {
			screen.ShowSide(false)
		} else {
			screen.ShowSide(true)
		}
		return true
	case termbox.KeyEsc:
		screen.ShowSide(true)
		return true
	case termbox.KeyCtrlF:
		dig.Mode = FindMode
		return true
	}
	switch ev.Ch {
	case '<':
		screen.ExpandSide(-1)
		return true
	case '>':
		screen.ExpandSide(1)
		return true
	}
	return false
}

// handleFind handles FindMode events.
// When the event was handled, it will return true.
func handleFind(ev termbox.Event) bool {
	switch ev.Key {
	case termbox.KeyEsc, termbox.KeyCtrlQ, termbox.KeyCtrlK:
		dig.FindString = ""
		dig.Mode = NormalMode
		return true
	case termbox.KeyEnter:
		from := nextIdx(dig.Commits, screen.Side.CurIdx)
		if idx := findByHash(dig.Commits, dig.FindString, from); idx != -1 {
			screen.Side.CurIdx = idx
		}
		if idx := findByWord(dig.Commits, dig.FindString, from); idx != -1 {
			screen.Side.CurIdx = idx
		}
		return true
	case termbox.KeyBackspace, termbox.KeyBackspace2:
		_, size := utf8.DecodeLastRuneInString(dig.FindString)
		dig.FindString = dig.FindString[:len(dig.FindString)-size]
		return true
	}
	dig.FindString += string(ev.Ch)
	return true
}

// nextIdx returns next index from commits.
// If reached the last commit index, it will return 0.
func nextIdx(commits []*Commit, i int) int {
	if i == len(commits)-1 {
		return 0
	}
	return i + 1
}

// findByHash finds a commit by hash.
func findByHash(commits []*Commit, hash string, from int) int {
	for i, c := range commits[from:] {
		if c.Hash == hash {
			return from + i
		}
	}
	for i, c := range commits[:from] {
		if c.Hash == hash {
			return i
		}
	}
	return -1
}

// findByWord finds next commit by word inside of title of commits.
func findByWord(commits []*Commit, word string, from int) int {
	for i, c := range commits[from:] {
		if strings.Contains(c.Title, word) {
			return from + i
		}
	}
	for i, c := range commits[:from] {
		if strings.Contains(c.Title, word) {
			return i
		}
	}
	return -1
}

func main() {
	up := flag.Bool("up", false, "dig up from initial commit (don't use with -down)")
	down := flag.Bool("down", false, "dig down from latest commit (don't use with -up)")
	repoDir := flag.String("C", ".", "git repository to dig")
	flag.Parse()

	var digUp bool
	if *up && *down {
		flag.Usage()
		os.Exit(2)
	} else if *down {
		digUp = false
	} else {
		digUp = true
	}

	commits, err := allCommits(*repoDir, digUp)
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not get commits: %v\n", err)
		os.Exit(1)
	}

	err = termbox.Init()
	if err != nil {
		fmt.Fprint(os.Stderr, err)
		os.Exit(1)
	}
	defer termbox.Close()

	w, h := termbox.Size()
	size := Pt{h, w}
	screen = NewScreen(size)

	dig = &Program{
		NormalMode,
		*repoDir,
		commits,
		"",
	}

	events := make(chan termbox.Event, 20)
	go func() {
		for {
			events <- termbox.PollEvent()
		}
	}()

	for {
		termbox.Clear(termbox.ColorDefault, termbox.ColorDefault)
		screen.Draw()
		termbox.Flush()

		ev := <-events
		switch ev.Type {
		case termbox.EventKey:
			// handle global event
			if dig.Mode == NormalMode {
				// exit handling could not be inside of a function.
				if ev.Key == termbox.KeyCtrlQ || ev.Ch == 'q' {
					return
				}
			}
			if dig.Mode == NormalMode {
				if ok := handleNormal(ev); ok {
					continue
				}
				screen.Side.Handle(ev)
				screen.Main.Handle(ev)
			} else if dig.Mode == FindMode {
				if ok := handleFind(ev); ok {
					continue
				}
			}
		case termbox.EventResize:
			// weird, but terminal(or termbox?) should be cleared
			// before checking the terminal size
			// when user changes the terminal window to fullscreen.
			termbox.Clear(termbox.ColorDefault, termbox.ColorDefault)
			w, h := termbox.Size()
			size := Pt{h, w}
			screen.Resize(size)
		}
	}
}
