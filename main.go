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

const kPieceMaxSize = 5
const kBoardHeight = 8
const kBoardWidth = 8

type PieceID int
type PieceField [kPieceMaxSize][kPieceMaxSize]PieceID
type BoardField [kBoardHeight][kBoardWidth]PieceID

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

func (p Positions) Len() int {
	return len(p)
}

func (p Positions) Swap(i, j int) {
	p[i], p[j] = p[j], p[i]
}

func (p Positions) Less(i, j int) bool {
	if p[i].x < p[j].x || (p[i].x == p[j].x && p[i].y < p[j].y) {
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

func (op *OrientedPiece) String() string {
	f := make([][]rune, op.h+2)
	for i := range f {
		f[i] = make([]rune, op.w+2)
	}
	var result bytes.Buffer
	for _, p := range op.border {
		f[p.x+1][p.y+1] = '○'
	}
	for _, p := range op.body {
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
	body_map := make(map[Position]struct{})
	border_map := make(map[Position]struct{})
	for i := range p {
		for j := range p[i] {
			if p[i][j] != 0 {
				if i+1 > op.h {
					op.h = i + 1
				}
				if j+1 > op.w {
					op.w = j + 1
				}
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

func (op *OrientedPiece) Copy() *OrientedPiece {
	new_op := *op
	new_op.body = make(Positions, len(op.body))
	copy(new_op.body, op.body)
	new_op.border = make(Positions, len(op.border))
	copy(new_op.border, op.border)
	return &new_op
}

func (op *OrientedPiece) Equals(rhs *OrientedPiece) bool {
	if len(op.body) != len(rhs.body) ||
		op.h != rhs.h || op.w != rhs.w {
		return false
	}
	for i := range op.body {
		if op.body[i] != rhs.body[i] {
			return false
		}
	}
	return true
}

func (op *OrientedPiece) Rotate() OrientedPiece {
	var rotated OrientedPiece
	rotated.h = op.w
	rotated.w = op.h
	for i := range op.body {
		rotated.body = append(rotated.body, Position{x: op.w - 1 - op.body[i].y, y: op.body[i].x})
	}
	sort.Sort(rotated.body)
	for i := range op.border {
		rotated.border = append(rotated.border, Position{x: op.w - 1 - op.border[i].y, y: op.border[i].x})
	}
	sort.Sort(rotated.border)
	return rotated
}

func (op *OrientedPiece) Mirror() OrientedPiece {
	var mirrored OrientedPiece
	mirrored.h = op.w
	mirrored.w = op.h
	for i := range op.body {
		mirrored.body = append(mirrored.body, Position{x: op.body[i].y, y: op.body[i].x})
	}
	sort.Sort(mirrored.body)
	for i := range op.border {
		mirrored.border = append(mirrored.border, Position{x: op.border[i].y, y: op.border[i].x})
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

func (p Piece) String() string {
	return fmt.Sprintf("id: %v\n%v\n", p.id, p.ops)
}

func NewPiece(p PieceField, id PieceID) *Piece {
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
	return &new_piece
}

func (p *Piece) Copy() *Piece {
	new_piece := Piece{id: p.id}
	new_piece.ops = make([]OrientedPiece, len(p.ops))
	for i := range new_piece.ops {
		new_piece.ops[i] = *p.ops[i].Copy()
	}
	return &new_piece
}

func (p *Piece) Matches(op *OrientedPiece) bool {
	for i := range p.ops {
		if p.ops[i].Equals(op) {
			return true
		}
	}
	return false
}

///////////////////////////////
// Board
///////////////////////////////

type Board struct {
	s           BoardField
	pieces      [NumberOfPieces]Piece
	used_pieces [NumberOfPieces]bool
	ch          chan *BoardField
	depth       int
}

// NewBoard returns an empty Board of the specified width and height.
func NewBoard(pieces *[NumberOfPieces]Piece, ch chan *BoardField) *Board {
	new_board := Board{pieces: *pieces, ch: ch}
	return &new_board
}

func (b *Board) Copy() *Board {
	new_board := *b
	for i := range new_board.pieces {
		new_board.pieces[i] = *b.pieces[i].Copy()
	}
	return &new_board
}

func (b *Board) Get(x, y int) PieceID {
	if b.Contains(x, y) {
		return b.s[x][y]
	} else {
		return PieceID(-1)
	}
}

func (b *Board) Contains(x, y int) bool {
	if x >= 0 && x < kBoardHeight && y >= 0 && y < kBoardWidth {
		return true
	}
	return false
}

func (b *Board) Insert(x, y int, op *OrientedPiece, id PieceID) (bool, *Positions) {
	if x < 0 || y < 0 || x+op.h > kBoardHeight || y+op.w > kBoardWidth {
		return false, nil
	}
	for _, p := range op.body {
		if b.Get(p.x+x, p.y+y) != 0 {
			return false, nil
		}
	}
	for _, p := range op.body {
		b.s[p.x+x][p.y+y] = id
	}
	var new_seeds Positions
	for _, p := range op.border {
		p2 := Position{p.x + x, p.y + y}
		if b.Contains(p2.x, p2.y) {
			new_seeds = append(new_seeds, p2)
		}
	}
	return true, &new_seeds
}

func (b *Board) Remove(x, y int, op *OrientedPiece) {
	for _, p := range op.body {
		b.s[p.x+x][p.y+y] = 0
	}
}

func (b *Board) CheckCorners() bool {
	// corner fields have additional conditions to skip equivalent solutions
	//fmt.Println(b)
	if b.s[0][0] < PieceID(NumberOfPieces-2) &&
		b.s[0][kBoardWidth-1] < PieceID(NumberOfPieces-1) &&
		b.s[kBoardHeight-1][0] < PieceID(NumberOfPieces) &&
		(b.s[0][kBoardWidth-1] == 0 || b.s[0][kBoardWidth-1] > b.s[0][0]) &&
		(b.s[kBoardHeight-1][0] == 0 || b.s[kBoardHeight-1][0] > b.s[0][kBoardWidth-1]) &&
		(b.s[kBoardHeight-1][kBoardWidth-1] == 0 || b.s[kBoardHeight-1][kBoardWidth-1] > b.s[kBoardHeight-1][0]) {
		return true
	}
	return false
}

//var count int = 0

func (b *Board) fillWithPiece(x, y, p_idx int, queue Positions) {
	// iterate over all oriented pieces
	piece := &b.pieces[p_idx]
	for i := range piece.ops {
		//fmt.Println(i)
		op := &piece.ops[i]
		// iterate over all body blocks
		for _, offset := range op.body {
			p := Position{x - offset.x, y - offset.y}
			ok, new_seeds := b.Insert(p.x, p.y, op, piece.id)
			if ok {
				//fmt.Println("\x0c")
				//fmt.Print("\x1b\x5b\x48\x1b\x5b\x32\x4a", b.s)
				if b.CheckCorners() {
					if b.depth == len(b.pieces)-1 {
						// solution found
						bf := b.s
						b.ch <- &bf
					} else {
						new_queue := append(queue, *new_seeds...)
						b.depth++
						b.FillPositions(new_queue)
						b.depth--
					}
				} else {
					//fmt.Println("skipped solution")
				}
				b.Remove(p.x, p.y, op)
			}
		}
	}
}

func (b *Board) FillPositions(queue Positions) {
	var seed Position
	for i := range queue {
		seed = queue[i]
		if b.s[seed.x][seed.y] == 0 {
			queue = queue[i+1:]
			break
		}
	}
	// we have a free seed now
	// rotate over all pieces
	for i := range b.used_pieces {
		if b.used_pieces[i] {
			continue
		}
		b.used_pieces[i] = true
		if b.depth == 0 && i < NumberOfPieces-3 && false {
			// top recursion level
			new_queue := make(Positions, len(queue))
			copy(new_queue, queue)
			fmt.Println("Spawn new goprocess for piece ", i)
			new_board := b.Copy()
			go new_board.fillWithPiece(seed.x, seed.y, i, new_queue)
		} else {
			// if b.depth == 1 {
			// 	fmt.Println("Fill with piece ", i)
			// }
			b.fillWithPiece(seed.x, seed.y, i, queue)
		}
		b.used_pieces[i] = false
	}
}

func (b *Board) Fill() {
	seed_init := Positions{Position{0, 0}}
	go b.FillPositions(seed_init)
}

// String returns the game board as a string.

func (bf *BoardField) String() string {
	var buf bytes.Buffer
	for x := range bf {
		for y := range bf[x] {
			c := "  "
			if bf[x][y] != 0 {
				c = string(bf.GetRune(x, y)) + " "
			}
			buf.WriteString(c)
		}
		buf.WriteByte('\n')
	}
	return buf.String()
}

func (bf *BoardField) GetRune(x, y int) rune {
	type N struct {
		up, right, down, left bool
	}
	var result rune
	var up, down, right, left bool
	id := bf[x][y]
	return 'A' - 1 + rune(id)
	return '🍅' - 1 + rune(id)
	if x > 0 && bf[x-1][y] == id {
		up = true
	}
	if x < len(bf)-1 && bf[x+1][y] == id {
		down = true
	}
	if y > 0 && bf[x][y-1] == id {
		left = true
	}
	if y < len(bf[x])-1 && bf[x][y+1] == id {
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
		pieces[i] = *NewPiece(pieces_init[i], id)
	}
	//	fmt.Println(pieces)
	var ch = make(chan *BoardField)
	b := NewBoard(&pieces, ch)
	//	fmt.Println(b)
	b.Fill()
	cnt := 0
	for {
		bf := <-ch
		fmt.Print("\x0c", bf)
		cnt++
		fmt.Println(cnt)
		if cnt == 100 {
			pprof.StopCPUProfile()
			os.Exit(0)
		}

	}
}
