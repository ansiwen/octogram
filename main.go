package main

import (
	"bytes"
	"fmt"
	"os"
	"sort"
	// "math/rand"
	// "time"
	"flag"
	"log"
	"runtime/pprof"
)

var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to file")

//const piece_size_max int = 5

type PieceID int
type PieceField [][]PieceID

// list of available pieces
var pieces_init = [...]PieceField{
	{
		{1, 1, 1, 1, 1},
	},
	{
		{1, 1, 1, 1},
		{1},
	},
	{
		{1, 1, 1},
		{1},
		{1},
	},
	{
		{1, 1, 1, 1},
		{0, 1},
	},
	{
		{1, 1},
		{0, 1, 1},
		{0, 1},
	},
	{
		{0, 1},
		{1, 1, 1},
		{0, 1},
	},
	{
		{1, 1, 1},
		{1, 1},
	},
	{
		{1},
		{1, 1, 1},
		{1},
	},
	{
		{0, 1, 1},
		{1, 1},
		{1},
	},
	{
		{1, 1},
		{1},
		{1, 1},
	},
	{
		{0, 1, 1, 1},
		{1, 1},
	},
	{
		{0, 1, 1},
		{0, 1},
		{1, 1},
	},
	{
		{1, 1},
		{1, 1},
	},
}

const NumberOfPieces = len(pieces_init)

///////////////////////////////
// Position
///////////////////////////////

type Position struct {
	x, y int
}

type Positions []Position

func (this Positions) Len() int {
	return len(this)
}

func (this Positions) Swap(i, j int) {
	this[i], this[j] = this[j], this[i]
}

func (this Positions) Less(i, j int) bool {
	if this[i].x < this[j].x || (this[i].x == this[j].x && this[i].y < this[j].y) {
		return true
	}
	return false
}

///////////////////////////////
// OrientedPiece
///////////////////////////////
type OrientedPiece struct {
	h, w         int
	body, border Positions
}

func (this OrientedPiece) String() string {
	f := make([][]rune, this.h+2)
	for i := range f {
		f[i] = make([]rune, this.w+2)
	}
	var result bytes.Buffer
	for _, p := range this.border {
		f[p.x+1][p.y+1] = '○'
	}
	for _, p := range this.body {
		f[p.x+1][p.y+1] = '●'
	}
	result.WriteString("\n")
	for _, l := range f {
		for _, c := range l {
			if c != 0 {
				result.WriteString(fmt.Sprintf("%c ", c))
			} else {
				result.WriteString("  ")
			}
		}
		result.WriteString("\n")
	}
	return result.String()
}

func NewOrientedPiece(p PieceField) OrientedPiece {
	var op OrientedPiece
	op.h = len(p)
	body_map := make(map[Position]struct{})
	border_map := make(map[Position]struct{})
	for i := range p {
		if len(p[i]) > op.w {
			op.w = len(p[i])
		}
		for j := range p[i] {
			if p[i][j] != 0 {
				body_map[Position{i, j}] = struct{}{}
				border_map[Position{i + 1, j}] = struct{}{}
				border_map[Position{i - 1, j}] = struct{}{}
				border_map[Position{i, j + 1}] = struct{}{}
				border_map[Position{i, j - 1}] = struct{}{}
			}
		}
	}
	op.body = make(Positions, len(body_map))
	i := 0
	for k := range body_map {
		op.body[i] = k
		i++
	}
	sort.Sort(op.body)
	for k := range border_map {
		_, is_body := body_map[k]
		if !is_body {
			op.border = append(op.border, k)
		}
	}
	sort.Sort(op.border)
	return op
}

func (this *OrientedPiece) Equals(rhs *OrientedPiece) bool {
	if len(this.body) != len(rhs.body) ||
		this.h != rhs.h || this.w != rhs.w {
		return false
	}
	for i := range this.body {
		if this.body[i] != rhs.body[i] {
			return false
		}
	}
	return true
}

func (this *OrientedPiece) Rotate() OrientedPiece {
	var rotated OrientedPiece
	rotated.h = this.w
	rotated.w = this.h
	for i := range this.body {
		rotated.body = append(rotated.body, Position{x: this.w - 1 - this.body[i].y, y: this.body[i].x})
	}
	sort.Sort(rotated.body)
	for i := range this.border {
		rotated.border = append(rotated.border, Position{x: this.w - 1 - this.border[i].y, y: this.border[i].x})
	}
	sort.Sort(rotated.border)
	return rotated
}

func (this *OrientedPiece) Mirror() OrientedPiece {
	var mirrored OrientedPiece
	mirrored.h = this.w
	mirrored.w = this.h
	for i := range this.body {
		mirrored.body = append(mirrored.body, Position{x: this.body[i].y, y: this.body[i].x})
	}
	sort.Sort(mirrored.body)
	for i := range this.border {
		mirrored.border = append(mirrored.border, Position{x: this.border[i].y, y: this.border[i].x})
	}
	sort.Sort(mirrored.border)
	return mirrored
}

///////////////////////////////
// Piece
///////////////////////////////
type Piece struct {
	ops []OrientedPiece
	id  PieceID
}

func (this Piece) String() string {
	return fmt.Sprintf("id: %v\n%v\n", this.id, this.ops)
}

func NewPiece(p PieceField, id PieceID) Piece {
	new_piece := Piece{id: id}
	op := NewOrientedPiece(p)
	new_piece.ops = append(new_piece.ops, op)
	op = op.Rotate()
	if !new_piece.Matches(&op) {
		new_piece.ops = append(new_piece.ops, op)
	}
	op = op.Rotate()
	if !new_piece.Matches(&op) {
		new_piece.ops = append(new_piece.ops, op)
	}
	op = op.Rotate()
	if !new_piece.Matches(&op) {
		new_piece.ops = append(new_piece.ops, op)
	}
	op = op.Mirror()
	if !new_piece.Matches(&op) {
		new_piece.ops = append(new_piece.ops, op)
	}
	op = op.Rotate()
	if !new_piece.Matches(&op) {
		new_piece.ops = append(new_piece.ops, op)
	}
	op = op.Rotate()
	if !new_piece.Matches(&op) {
		new_piece.ops = append(new_piece.ops, op)
	}
	op = op.Rotate()
	if !new_piece.Matches(&op) {
		new_piece.ops = append(new_piece.ops, op)
	}
	return new_piece
}

func (this *Piece) Matches(op *OrientedPiece) bool {
	for i := range this.ops {
		if this.ops[i].Equals(op) {
			return true
		}
	}
	return false
}

///////////////////////////////
// Board
///////////////////////////////

type Board struct {
	w, h, depth      int
	s                PieceField
	pieces           [NumberOfPieces]Piece
	off_board_pieces [NumberOfPieces]bool
}

// NewBoard returns an empty Board of the specified width and height.
func NewBoard(w, h int, pieces [NumberOfPieces]Piece) Board {
	b := Board{w: w, h: h}
	b.s = make(PieceField, h)
	for i := range b.s {
		b.s[i] = make([]PieceID, w)
	}
	b.pieces = pieces
	//	b.off_board_pieces = make([]bool, len(pieces))
	for i := range pieces {
		b.off_board_pieces[i] = true
	}
	return b
}

func (this *Board) Copy() *Board {
	new_board := Board{h: this.h, w: this.w, depth: this.depth}
	new_board.s = this.s.Copy()
	new_board.pieces = this.pieces
	//	new_board.off_board_pieces = make([]bool, len(this.off_board_pieces))
	//	copy(new_board.off_board_pieces, this.off_board_pieces)
	new_board.off_board_pieces = this.off_board_pieces
	return &new_board
}

func (this *Board) Get(x, y int) PieceID {
	if this.Contains(x, y) {
		return this.s[x][y]
	} else {
		return PieceID(-1)
	}
}

func (this *Board) Contains(x, y int) bool {
	if x >= 0 && x < this.h && y >= 0 && y < this.w {
		return true
	}
	return false
}

func (this *Board) Insert(x, y int, op *OrientedPiece, id PieceID) (bool, *Positions) {
	if x < 0 || y < 0 || x+op.h > this.h || y+op.w > this.w {
		return false, nil
	}
	for _, p := range op.body {
		if this.Get(p.x+x, p.y+y) != 0 {
			return false, nil
		}
	}
	for _, p := range op.body {
		this.s[p.x+x][p.y+y] = id
	}
	var new_seeds Positions
	for _, p := range op.border {
		p2 := Position{p.x + x, p.y + y}
		if this.Contains(p2.x, p2.y) {
			new_seeds = append(new_seeds, p2)
		}
	}
	return true, &new_seeds
}

func (this *Board) Remove(x, y int, op *OrientedPiece) {
	for _, p := range op.body {
		this.s[p.x+x][p.y+y] = 0
	}
}

func (this *Board) CheckCorners() bool {
	// corner fields have additional conditions to skip equivalent solutions
	if this.s[0][0] < PieceID(NumberOfPieces-2) &&
		this.s[0][this.w-1] < PieceID(NumberOfPieces-1) &&
		this.s[this.h-1][0] < PieceID(NumberOfPieces) &&
		(this.s[0][this.w-1] == 0 || this.s[0][this.w-1] > this.s[0][0]) &&
		(this.s[this.h-1][0] == 0 || this.s[this.h-1][0] > this.s[0][this.w-1]) &&
		(this.s[this.h-1][this.w-1] == 0 || this.s[this.h-1][this.w-1] > this.s[this.h-1][0]) {
		return true
	}
	return false
}

//var count int = 0

func (this *Board) fillWithPiece(x, y, p_idx int, queue Positions) {
	// iterate over all oriented pieces
	piece := &this.pieces[p_idx]
	for i := range piece.ops {
		//fmt.Println(i)
		op := &piece.ops[i]
		// iterate over all body blocks
		for _, offset := range op.body {
			p := Position{x - offset.x, y - offset.y}
			ok, new_seeds := this.Insert(p.x, p.y, op, piece.id)
			if ok {
				//fmt.Println("\x0c")
				//fmt.Print("\x1b\x5b\x48\x1b\x5b\x32\x4a", this.s)
				if this.CheckCorners() {
					if this.depth == len(this.pieces)-1 {
						// solution found
						ch <- this.s.Copy()
					} else {
						new_queue := append(queue, *new_seeds...)
						this.depth++
						this.FillPositions(new_queue)
						this.depth--
					}
				} else {
					//fmt.Println("skipped solution")
				}
				this.Remove(p.x, p.y, op)
			}
		}
	}
}

func (this *Board) FillPositions(queue Positions) {
	var seed Position
	for i := range queue {
		seed = queue[i]
		if this.s[seed.x][seed.y] == 0 {
			queue = queue[i+1:]
			break
		}
	}
	// we have a free seed now
	// rotate over all pieces
	for i := range this.off_board_pieces {
		if !this.off_board_pieces[i] {
			continue
		}
		this.off_board_pieces[i] = false
		if this.depth == 0 && i < NumberOfPieces-3 && false {
			// top recursion level
			new_queue := make(Positions, len(queue))
			copy(new_queue, queue)
			fmt.Println("Spawn new goprocess for piece ", i)
			new_board := this.Copy()
			go new_board.fillWithPiece(seed.x, seed.y, i, new_queue)
		} else {
			// if this.depth == 1 {
			// 	fmt.Println("Fill with piece ", i)
			// }
			this.fillWithPiece(seed.x, seed.y, i, queue)
		}
		this.off_board_pieces[i] = true
	}
}

func (this *Board) Fill() {
	seed_init := Positions{Position{0, 0}}
	go this.FillPositions(seed_init)
}

// String returns the game board as a string.

func (pf PieceField) String() string {
	var buf bytes.Buffer
	for x := range pf {
		for y := range pf[x] {
			c := "  "
			if pf[x][y] != 0 {
				c = string(pf.GetRune(x, y)) + " "
			}
			buf.WriteString(c)
		}
		buf.WriteByte('\n')
	}
	return buf.String()
}

func (pf PieceField) Copy() PieceField {
	cpy := make(PieceField, len(pf))
	for i := range cpy {
		cpy[i] = make([]PieceID, len(pf[i]))
		copy(cpy[i], pf[i])
	}
	return cpy
}

func (pf PieceField) GetRune(x, y int) rune {
	type N struct {
		up, right, down, left bool
	}
	var result rune
	var up, down, right, left bool
	id := pf[x][y]
	return 'A' - 1 + rune(id)
	return '🍅' - 1 + rune(id)
	if x > 0 && pf[x-1][y] == id {
		up = true
	}
	if x < len(pf)-1 && pf[x+1][y] == id {
		down = true
	}
	if y > 0 && pf[x][y-1] == id {
		left = true
	}
	if y < len(pf[x])-1 && pf[x][y+1] == id {
		right = true
	}
	switch (N{up, right, down, left}) {
	case N{true, false, false, false}:
		result = '╹'
	case N{false, true, false, false}:
		result = '╺'
	case N{false, false, true, false}:
		result = '╻'
	case N{false, false, false, true}:
		result = '╸'
	case N{true, true, false, false}:
		result = '┗'
	case N{true, false, true, false}:
		result = '┃'
	case N{true, false, false, true}:
		result = '┛'
	case N{false, true, true, false}:
		result = '┏'
	case N{false, true, false, true}:
		result = '━'
	case N{false, false, true, true}:
		result = '┓'
	case N{true, true, true, false}:
		result = '┣'
	case N{true, true, false, true}:
		result = '┻'
	case N{true, false, true, true}:
		result = '┫'
	case N{false, true, true, true}:
		result = '┳'
	case N{true, true, true, true}:
		result = '╋'
	}
	return result
}

var ch = make(chan PieceField)

func main() {
	flag.Parse()
	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	var pieces [NumberOfPieces]Piece
	for i := range pieces_init {
		id := PieceID(i + 1)
		pieces[i] = NewPiece(pieces_init[i], id)
	}
	//	fmt.Println(pieces)
	b := NewBoard(8, 8, pieces)
	//	fmt.Println(b)
	b.Fill()
	cnt := 0
	for {
		pf := <-ch
		fmt.Print("\x0c", pf)
		cnt++
		fmt.Println(cnt)
		if cnt == 100 {
			pprof.StopCPUProfile()
			os.Exit(0)
		}

	}
}
