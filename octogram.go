package main

import (
	"bytes"
	"flag"
	"fmt"
	"runtime"
	"sort"
	"sync"
)

var concurdepth = flag.Int("d", -1, "enable concurrency in given recursion depth")
var stopcount = flag.Int("c", 0, "stop after that many solutions")
var numberofcpus = flag.Int("j", 1, "how many CPU cores to be used")

const kPieceMaxSize = 5
const kBoardHeight = 8
const kBoardWidth = 8

type PieceID int
type PieceField [kPieceMaxSize][kPieceMaxSize]PieceID

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

const kNumberOfPieces = len(pieces_init)

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

func (op OrientedPiece) String() string {
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
	ops  []OrientedPiece
	id   PieceID
	used bool
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
	new_piece := Piece{id: p.id, used: p.used}
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
// BoardField
///////////////////////////////
type BoardField [kBoardHeight][kBoardWidth]PieceID

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
	return '🍅' - 1 + rune(bf[x][y])
}

///////////////////////////////
// Board
///////////////////////////////
type Board struct {
	s      BoardField
	pieces [kNumberOfPieces]Piece
	ch     chan *BoardField
	depth  int
	wg     sync.WaitGroup
}

// NewBoard returns an empty Board of the specified width and height.
func NewBoard(pieces *[kNumberOfPieces]Piece, ch chan *BoardField) *Board {
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

func (b *Board) Insert(x, y int, op *OrientedPiece, id PieceID, queue *Positions) (bool, *Positions) {
	if x < 0 || y < 0 || x+op.h > kBoardHeight || y+op.w > kBoardWidth {
		return false, nil
	}
	for _, p := range op.body {
		if b.s[p.x+x][p.y+y] != 0 {
			return false, nil
		}
	}
	for _, p := range op.body {
		b.s[p.x+x][p.y+y] = id
	}
	var new_queue = *queue
	for i := range op.border {
		p := Position{op.border[i].x + x, op.border[i].y + y}
		if p.x >= 0 && p.x < kBoardHeight && p.y >= 0 && p.y < kBoardWidth {
			new_queue = append(new_queue, p)
		}
	}
	return true, &new_queue
}

func (b *Board) Remove(x, y int, op *OrientedPiece) {
	for _, p := range op.body {
		b.s[p.x+x][p.y+y] = 0
	}
}

func (b *Board) CheckCorners() bool {
	// corner fields have additional conditions to skip equivalent solutions:
	// upper left must be smallest, and upper right must by smaller than lower left
	if b.s[0][0] < PieceID(kNumberOfPieces-2) &&
		b.s[0][kBoardWidth-1] < PieceID(kNumberOfPieces) &&
		(b.s[0][kBoardWidth-1] == 0 || b.s[0][kBoardWidth-1] > b.s[0][0]) &&
		(b.s[kBoardHeight-1][0] == 0 || b.s[kBoardHeight-1][0] > b.s[0][kBoardWidth-1]) &&
		(b.s[kBoardHeight-1][kBoardWidth-1] == 0 || b.s[kBoardHeight-1][kBoardWidth-1] > b.s[0][0]) {
		return true
	}
	return false
}

func (b *Board) fillWithPiece(x, y, p_idx int, queue Positions) {
	// iterate over all oriented pieces
	piece := &b.pieces[p_idx]
	for i := range piece.ops {
		op := &piece.ops[i]
		// iterate over all body blocks
		for _, offset := range op.body {
			p := Position{x - offset.x, y - offset.y}
			ok, new_queue := b.Insert(p.x, p.y, op, piece.id, &queue)
			if ok {
				if b.CheckCorners() {
					if b.depth == kNumberOfPieces-1 {
						// solution found
						bf := b.s
						b.ch <- &bf
					} else {
						b.depth++
						b.FillPositions(*new_queue)
						b.depth--
					}
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
	for i := range b.pieces {
		if b.pieces[i].used {
			continue
		}
		if b.depth == 0 && i != 9 {
			continue
		}
		b.pieces[i].used = true
		if b.depth == *concurdepth {
			// spawn goroutines at this level
			new_queue := make(Positions, len(queue))
			copy(new_queue, queue)
			new_board := b.Copy()
			b.wg.Add(1)
			// make copies of i and seed for local scope
			i := i
			seed := seed
			go func() {
				defer b.wg.Done()
				new_board.fillWithPiece(seed.x, seed.y, i, new_queue)
			}()
		} else {
			b.fillWithPiece(seed.x, seed.y, i, queue)
		}
		b.pieces[i].used = false
	}
}

func (b *Board) Fill() {
	seed_init := Positions{Position{0, 0}}
	go func() {
		b.FillPositions(seed_init)
		b.wg.Wait()
		b.ch <- nil
	}()
}

///////////////////////////////
// Main
///////////////////////////////
func main() {
	flag.Parse()
	runtime.GOMAXPROCS(*numberofcpus)
	var pieces [kNumberOfPieces]Piece
	for i := range pieces_init {
		id := PieceID(i + 1)
		pieces[i] = *NewPiece(pieces_init[i], id)
	}
	fmt.Println(pieces)
	var ch = make(chan *BoardField)
	b := NewBoard(&pieces, ch)
	b.Fill()
	cnt := 0
	for {
		bf := <-ch
		if bf == nil {
			// all solver terminated
			break
		}
		cnt++
		fmt.Println("Solution ", cnt)
		fmt.Println(bf)
		if cnt == *stopcount {
			break
		}
	}
}
