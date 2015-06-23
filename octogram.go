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

type pieceID int
type pieceField [kPieceMaxSize][kPieceMaxSize]pieceID

// list of available pieces
var pieces_init = [...]pieceField{
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
// position
///////////////////////////////
type position struct {
	x, y int
}

type positions []position

func (p positions) Len() int {
	return len(p)
}

func (p positions) Swap(i, j int) {
	p[i], p[j] = p[j], p[i]
}

func (p positions) Less(i, j int) bool {
	if p[i].x < p[j].x || (p[i].x == p[j].x && p[i].y < p[j].y) {
		return true
	}
	return false
}

///////////////////////////////
// orientedPiece
///////////////////////////////
type orientedPiece struct {
	h, w         int
	body, border positions
}

func (op orientedPiece) String() string {
	var result bytes.Buffer
	f := make([][]rune, op.h+2)
	for i := range f {
		f[i] = make([]rune, op.w+2)
	}
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

func newOrientedPiece(p pieceField) orientedPiece {
	var op orientedPiece
	body_map := make(map[position]struct{})
	border_map := make(map[position]struct{})
	for i := range p {
		for j := range p[i] {
			if p[i][j] != 0 {
				if i+1 > op.h {
					op.h = i + 1
				}
				if j+1 > op.w {
					op.w = j + 1
				}
				body_map[position{i, j}] = struct{}{}
				border_map[position{i + 1, j}] = struct{}{}
				border_map[position{i - 1, j}] = struct{}{}
				border_map[position{i, j + 1}] = struct{}{}
				border_map[position{i, j - 1}] = struct{}{}
			}
		}
	}
	op.body = make(positions, len(body_map))
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

func (op *orientedPiece) copy() *orientedPiece {
	new_op := *op
	new_op.body = make(positions, len(op.body))
	copy(new_op.body, op.body)
	new_op.border = make(positions, len(op.border))
	copy(new_op.border, op.border)
	return &new_op
}

func (op *orientedPiece) equals(rhs *orientedPiece) bool {
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

func (op *orientedPiece) rotate() orientedPiece {
	var rotated orientedPiece
	rotated.h = op.w
	rotated.w = op.h
	for i := range op.body {
		rotated.body = append(rotated.body, position{x: op.w - 1 - op.body[i].y, y: op.body[i].x})
	}
	sort.Sort(rotated.body)
	for i := range op.border {
		rotated.border = append(rotated.border, position{x: op.w - 1 - op.border[i].y, y: op.border[i].x})
	}
	sort.Sort(rotated.border)
	return rotated
}

func (op *orientedPiece) mirror() orientedPiece {
	var mirrored orientedPiece
	mirrored.h = op.w
	mirrored.w = op.h
	for i := range op.body {
		mirrored.body = append(mirrored.body, position{x: op.body[i].y, y: op.body[i].x})
	}
	sort.Sort(mirrored.body)
	for i := range op.border {
		mirrored.border = append(mirrored.border, position{x: op.border[i].y, y: op.border[i].x})
	}
	sort.Sort(mirrored.border)
	return mirrored
}

///////////////////////////////
// piece
///////////////////////////////
type piece struct {
	ops  []orientedPiece
	id   pieceID
	used bool
}

func (p piece) String() string {
	return fmt.Sprintf("id: %v\n%v\n", p.id, p.ops)
}

func newPiece(p pieceField, id pieceID) *piece {
	new_piece := piece{id: id}
	op := newOrientedPiece(p)
	new_piece.ops = append(new_piece.ops, op)
	op = op.rotate()
	if !new_piece.matches(&op) {
		new_piece.ops = append(new_piece.ops, op)
	}
	op = op.rotate()
	if !new_piece.matches(&op) {
		new_piece.ops = append(new_piece.ops, op)
	}
	op = op.rotate()
	if !new_piece.matches(&op) {
		new_piece.ops = append(new_piece.ops, op)
	}
	op = op.mirror()
	if !new_piece.matches(&op) {
		new_piece.ops = append(new_piece.ops, op)
	}
	op = op.rotate()
	if !new_piece.matches(&op) {
		new_piece.ops = append(new_piece.ops, op)
	}
	op = op.rotate()
	if !new_piece.matches(&op) {
		new_piece.ops = append(new_piece.ops, op)
	}
	op = op.rotate()
	if !new_piece.matches(&op) {
		new_piece.ops = append(new_piece.ops, op)
	}
	return &new_piece
}

func (p *piece) copy() *piece {
	new_piece := piece{id: p.id, used: p.used}
	new_piece.ops = make([]orientedPiece, len(p.ops))
	for i := range new_piece.ops {
		new_piece.ops[i] = *p.ops[i].copy()
	}
	return &new_piece
}

func (p *piece) matches(op *orientedPiece) bool {
	for i := range p.ops {
		if p.ops[i].equals(op) {
			return true
		}
	}
	return false
}

///////////////////////////////
// BoardField
///////////////////////////////
type BoardField [kBoardHeight][kBoardWidth]pieceID

// String returns the game board as a string.
func (bf *BoardField) String() string {
	var buf bytes.Buffer
	for x := range bf {
		for y := range bf[x] {
			c := "  "
			if bf[x][y] != 0 {
				c = string(bf.getRune(x, y)) + " "
			}
			buf.WriteString(c)
		}
		buf.WriteByte('\n')
	}
	return buf.String()
}

func (bf *BoardField) getRune(x, y int) rune {
	return '🍅' - 1 + rune(bf[x][y])
}

///////////////////////////////
// board
///////////////////////////////
type board struct {
	f      BoardField
	pieces [kNumberOfPieces]piece
	queue  positions
	pos    int
	ch     chan *BoardField
	depth  int
	wg     sync.WaitGroup
}

// newBoard returns an empty board of the specified width and height.
func newBoard(pieces *[kNumberOfPieces]piece, ch chan *BoardField) *board {
	new_board := board{pieces: *pieces, ch: ch}
	return &new_board
}

func (b *board) copy() *board {
	new_board := *b
	for i := range new_board.pieces {
		new_board.pieces[i] = *b.pieces[i].copy()
	}
	// don't copy queue
	return &new_board
}

func (b *board) set(x, y int, op *orientedPiece, id pieceID) {
	for _, p := range op.body {
		b.f[p.x+x][p.y+y] = id
	}
}

func (b *board) checkCorners() bool {
	// corner fields have additional conditions to skip equivalent solutions:
	// upper left must be smallest, and upper right must by smaller than lower left
	if b.f[0][0] < pieceID(kNumberOfPieces-2) &&
		b.f[0][kBoardWidth-1] < pieceID(kNumberOfPieces) &&
		(b.f[0][kBoardWidth-1] == 0 || b.f[0][kBoardWidth-1] > b.f[0][0]) &&
		(b.f[kBoardHeight-1][0] == 0 || b.f[kBoardHeight-1][0] > b.f[0][kBoardWidth-1]) &&
		(b.f[kBoardHeight-1][kBoardWidth-1] == 0 || b.f[kBoardHeight-1][kBoardWidth-1] > b.f[0][0]) {
		return true
	}
	return false
}

func (b *board) fillWithPiece(x, y, p_idx, q_idx int) {
	piece := &b.pieces[p_idx]
	// iterate over all oriented pieces
	for i := range piece.ops {
		op := &piece.ops[i]
		// iterate over all body blocks as possible offsets
	OffsetLoop:
		for _, offset := range op.body {
			p := position{x - offset.x, y - offset.y}
			old_q_len := len(b.queue)
			if p.x < 0 || p.y < 0 || p.x+op.h > kBoardHeight || p.y+op.w > kBoardWidth {
				continue OffsetLoop
			}
			// piece is inside board
			for _, p2 := range op.body {
				if b.f[p.x+p2.x][p.y+p2.y] != 0 {
					// block is not free, try next offset
					continue OffsetLoop
				}
			}
			// every block free, insert piece into board now
			b.set(p.x, p.y, op, piece.id)
			// insert free border blocks into fill queue
			for i := range op.border {
				p2 := position{op.border[i].x + p.x, op.border[i].y + p.y}
				if p2.x >= 0 && p2.x < kBoardHeight && p2.y >= 0 && p2.y < kBoardWidth &&
					b.f[p2.x][p2.y] == 0 {
					b.queue = append(b.queue, p2)
				}
			}
			if b.checkCorners() {
				if b.depth == kNumberOfPieces-1 {
					// solution found
					bf := b.f
					b.ch <- &bf
				} else {
					b.depth++
					b.fillPositions(q_idx)
					b.depth--
				}
			}
			b.set(p.x, p.y, op, 0)
			b.queue = b.queue[0:old_q_len]
		}
	}
}

func (b *board) fillPositions(q_idx int) {
	var seed position
	for i := q_idx; i < len(b.queue); i++ {
		seed = b.queue[i]
		if b.f[seed.x][seed.y] == 0 {
			q_idx = i + 1
			break
		}
	}
	// we have a free seed now
	// rotate over all pieces
	for i := range b.pieces {
		if b.pieces[i].used {
			continue
		}
		b.pieces[i].used = true
		if b.depth == *concurdepth {
			// spawn goroutines at this level
			new_board := b.copy()
			new_board.queue = make(positions, len(b.queue)-q_idx)
			copy(new_board.queue, b.queue[q_idx:])
			// make copies of i and seed for local scope
			i := i
			seed := seed
			b.wg.Add(1)
			go func() {
				defer b.wg.Done()
				new_board.fillWithPiece(seed.x, seed.y, i, 0)
			}()
		} else {
			b.fillWithPiece(seed.x, seed.y, i, q_idx)
		}
		b.pieces[i].used = false
	}
}

func (b *board) fill() {
	b.queue = positions{position{0, 0}}
	go func() {
		b.fillPositions(0)
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
	var pieces [kNumberOfPieces]piece
	for i := range pieces_init {
		id := pieceID(i + 1)
		pieces[i] = *newPiece(pieces_init[i], id)
	}
	fmt.Println(pieces)
	var ch = make(chan *BoardField)
	b := newBoard(&pieces, ch)
	b.fill()
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
