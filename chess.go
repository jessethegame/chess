package main

import (
	"fmt"
	"github.com/jessethegame/colorgrid"
)

type coords struct {
	x, y int
}

func (c coords) String() string {
	if c.x < 0 || 7 < c.x || c.y < 0 || 7 < c.y {
		panic(fmt.Sprint("Illegal coordinates (%d, %d)", c.x, c.y))
	}
	return fmt.Sprintf("%c%d", "ABCDEFGH"[c.x], c.y+1)
}

func (c *coords) Scan(state fmt.ScanState, verb rune) error {
	rx, _, _ := state.ReadRune()
	ry, _, _ := state.ReadRune()
	if rx < 'A' || 'G' < rx || ry < '1' || '8' < ry {
		return fmt.Errorf("Illegal chess coordinates: <%c, %c>", rx, ry)
	}
	c.x = int(rx - 'A')
	c.y = int(ry - '1')
	return nil
}

type pieceBareType int

const (
	PAWN pieceBareType = iota
	KNIGHT
	BISHOP
	ROOK
	QUEEN
	KING
)

type pieceColor int

const (
	BLACK pieceColor = iota
	WHITE
)

type pieceType struct {
	t pieceBareType
	c pieceColor
}

func (pt pieceType) String() string {
	switch pt.c {
	case WHITE:
		switch pt.t {
		case PAWN:
			return "♙"
		case KNIGHT:
			return "♘"
		case BISHOP:
			return "♗"
		case ROOK:
			return "♖"
		case QUEEN:
			return "♕"
		case KING:
			return "♔"
		}
		break
	case BLACK:
		switch pt.t {
		case PAWN:
			return "♟"
		case KNIGHT:
			return "♞"
		case BISHOP:
			return "♝"
		case ROOK:
			return "♜"
		case QUEEN:
			return "♛"
		case KING:
			return "♚"
		}
		break
	}
	panic("Illegal piece type")
}

// Operations on pieces
type pop interface{}

type popGetCoords chan<- coords

type popSetCoords coords

// Die. Close this channel when operation acknowledged (for sync)
type popKill chan<- bool

// Subscribe to moves by request all coordinates updates be sent down here.
// Send nil channel to cancel.
type popMoveCallback chan<- coords

type popSetType pieceType

type popGetType chan<- pieceType

type piece chan<- pop

// Operations on a chess board
type bop interface{}

// Place a new piece on the board
type bopNewPiece struct {
	coords
	ctrl chan<- pop
}

type bopMovePiece struct {
	from, to coords
}

type bopGetAllPieces chan<- piece

type bopDelPiece piece

type board chan<- bop

// Control operations are read from the control channel.
func spawnPiece(c <-chan pop) {
	var x, y int
	var movechan chan<- coords
	defer func() {
		if movechan != nil {
			close(movechan)
		}
	}()
	var pt pieceType
	for op := range c {
		switch t := op.(type) {
		case popSetCoords:
			x = t.x
			y = t.y
			if movechan != nil {
				movechan <- coords(t)
			}
		case popGetCoords:
			t <- coords{x, y}
			close(t)
		case popKill:
			close(t)
			return
		case popMoveCallback:
			movechan = t
		case popSetType:
			pt = pieceType(t)
		case popGetType:
			t <- pt
			close(t)
		default:
			panic(fmt.Sprintf("Illegal operation: %v", op))
		}
	}
}

func addPiece(x, y int, pt pieceType, b board) piece {
	// Start a piece
	c := make(chan pop)
	go spawnPiece(c)
	// Make it a pawn
	c <- popSetType(pt)
	// Move it to the desired coordinates
	c <- popSetCoords{x, y}
	b <- bopNewPiece{coords: coords{x, y}, ctrl: c}
	// piece will push updates to coordinates down this channel
	coordUpdates := make(chan coords)
	c <- popMoveCallback(coordUpdates)
	// Translate those updates to a message that includes the control channel
	go func() {
		for xy := range coordUpdates {
			b <- bopNewPiece{xy, c}
		}
	}()
	return c
}

func addPawn(x int, color pieceColor, b board) piece {
	var y int
	if color == WHITE {
		y = 1
	} else {
		y = 6
	}
	return addPiece(x, y, pieceType{PAWN, color}, b)
}

func baseline(color pieceColor) int {
	if color == WHITE {
		return 0
	}
	return 7
}

func addKnight(x int, color pieceColor, b board) piece {
	return addPiece(x, baseline(color), pieceType{KNIGHT, color}, b)
}

func addBishop(x int, color pieceColor, b board) piece {
	return addPiece(x, baseline(color), pieceType{BISHOP, color}, b)
}

func addRook(x int, color pieceColor, b board) piece {
	return addPiece(x, baseline(color), pieceType{ROOK, color}, b)
}

func addQueen(color pieceColor, b board) piece {
	return addPiece(3, baseline(color), pieceType{QUEEN, color}, b)
}

func addKing(color pieceColor, b board) piece {
	return addPiece(4, baseline(color), pieceType{KING, color}, b)
}

func draw(pc *pieceType, c coords) {
	var fg, bg colorgrid.Color
	var sprite string
	if (c.x%2 == 0 && c.y%2 == 0) || (c.x%2 == 1 && c.y%2 == 1) {
		bg = colorgrid.WHITE
	} else {
		bg = colorgrid.BLACK
	}
	if pc == nil {
		sprite = " "
		fg = colorgrid.BLACK
	} else {
		if pc.c == BLACK {
			fg = colorgrid.RED
		} else {
			fg = colorgrid.GREEN
		}
		sprite = pc.String()
	}

	colorgrid.Grid{colorgrid.Size{5, 3}}.Render(c.x, c.y, sprite, fg, bg)
}

// Run a board management unit.  Closes the done channel when all updates have
// been consumed and the input channel is closed (for sync).
func runBoard(c <-chan bop, done chan<- bool) {
	pieces := map[coords]piece{}
	for o := range c {
		switch t := o.(type) {
		case bopNewPiece:
			if _, exists := pieces[t.coords]; exists {
				panic(fmt.Sprintf("A piece already exists on %s", t.coords))
			}
			pieces[t.coords] = t.ctrl
			pc := make(chan pieceType)
			t.ctrl <- popGetType(pc)
			ptr := <-pc
			draw(&ptr, t.coords)
			//fmt.Printf("New piece: %s on %s\n", <-pc, t.coords)
			break
		case bopMovePiece:
			p, exists := pieces[t.from]
			if !exists {
				panic(fmt.Sprintf("No piece at %s", t.from))
			}
			delete(pieces, t.from)
			pieces[t.to] = p
			pc := make(chan pieceType)
			p <- popGetType(pc)
			ptr := <-pc
			draw(nil, t.from)
			draw(&ptr, t.to)
			//fmt.Printf("Move: %s from %s to %s\n", <-pc, t.from, t.to)
		case bopGetAllPieces:
			for _, p := range pieces {
				t <- p
			}
			close(t)
			break
		case bopDelPiece:
			donec := make(chan bool)
			cc := make(chan coords)
			t <- popGetCoords(cc)
			coords := <-cc
			t <- popKill(donec)
			<-donec
			delete(pieces, coords)
			//fmt.Printf("Deleted piece from %s\n", coords)
			break
		default:
			panic(fmt.Sprintf("Illegal board operation: %v", o))
		}
	}
	close(done)
}

func initBoard1p(b board, color pieceColor) {
	addPawn(0, color, b)
	addPawn(1, color, b)
	addPawn(2, color, b)
	addPawn(3, color, b)
	addPawn(4, color, b)
	addPawn(5, color, b)
	addPawn(6, color, b)
	addPawn(7, color, b)
	addRook(0, color, b)
	addKnight(1, color, b)
	addBishop(2, color, b)
	addQueen(color, b)
	addKing(color, b)
	addBishop(5, color, b)
	addKnight(6, color, b)
	addRook(7, color, b)
}

// Initialize an empty chess board by putting pieces in the right places
func initBoard(b board) {
	initBoard1p(b, WHITE)
	initBoard1p(b, BLACK)
}

func clearBoard(b board) {
	piecesc := make(chan piece)
	b <- bopGetAllPieces(piecesc)
	// Two-step to avoid dead-lock
	pieces := []piece{}
	for p := range piecesc {
		pieces = append(pieces, p)
	}
	for _, p := range pieces {
		b <- bopDelPiece(p)
	}
	return
}

// Parse human-readable coordinates into a move operation
func parseMoveOp(from, to string) (op bopMovePiece, err error) {
	_, err = fmt.Sscan(from, &op.from)
	if err != nil {
		return
	}
	_, err = fmt.Sscan(to, &op.to)
	return
}

func main() {
	g := colorgrid.Grid{colorgrid.Size{5, 3}}
	colorgrid.Clear()
	var x, y int
	for y = 0; y < 8; y++ {
		for x = 0; x < 8; x++ {
			if (x%2 == 0 && y%2 == 0) || (x%2 == 1 && y%2 == 1) {
				g.Render(x, y, " ", colorgrid.BLACK, colorgrid.WHITE)
			} else {
				g.Render(x, y, " ", colorgrid.WHITE, colorgrid.BLACK)
			}
		}
	}

	boardc := make(chan bop)
	boarddone := make(chan bool)
	go runBoard(boardc, boarddone)
	initBoard(boardc)
	// Open with a white pawn
	op, err := parseMoveOp("D2", "D4")
	if err != nil {
		panic(err.Error())
	}
	boardc <- op
	clearBoard(boardc)
}
