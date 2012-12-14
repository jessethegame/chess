package main

import (
	"fmt"
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

type bopSetPiece struct {
	coords
	ctrl chan<- pop
}

type bopGetAllPieces chan<- piece

type bopDelPiece piece

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

func addPawn(x, y int, color pieceColor, mu chan<- bop) piece {
	// Start a piece
	c := make(chan pop)
	go spawnPiece(c)
	// Make it a pawn
	c <- popSetType(pieceType{PAWN, color})
	// Move it to the desired coordinates
	c <- popSetCoords{x, y}
	mu <- bopSetPiece{coords: coords{x, y}, ctrl: c}
	// piece will push updates to coordinates down this channel
	coordUpdates := make(chan coords)
	c <- popMoveCallback(coordUpdates)
	// Translate those updates to a message that includes the control channel
	go func() {
		for xy := range coordUpdates {
			mu <- bopSetPiece{xy, c}
		}
	}()
	return c
}

// Run a board management unit. Push all location changes down this channel.
// Closes the done channel when all updates have been consumed and the input
// channel is closed (for sync).
func runBoard(c <-chan bop, done chan<- bool) {
	pieces := map[coords]piece{}
	for o := range c {
		switch t := o.(type) {
		case bopSetPiece:
			cc := make(chan coords)
			t.ctrl <- popGetCoords(cc)
			pieces[<-cc] = t.ctrl
			pc := make(chan pieceType)
			t.ctrl <- popGetType(pc)
			fmt.Printf("New piece: %s on %s\n", <-pc, t.coords)
			break
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
			fmt.Printf("Deleted piece from %s\n", coords)
			break
		default:
			panic(fmt.Sprintf("Illegal board operation: %v", o))
		}
	}
	close(done)
}

// Initialize an empty chess board by putting pieces in the right places
func initBoard(c chan<- bop) {
	addPawn(0, 1, WHITE, c)
	addPawn(1, 1, WHITE, c)
	addPawn(2, 1, WHITE, c)
	addPawn(3, 1, WHITE, c)
	addPawn(4, 1, WHITE, c)
	addPawn(5, 1, WHITE, c)
	addPawn(6, 1, WHITE, c)
	addPawn(7, 1, WHITE, c)
	addPawn(0, 6, BLACK, c)
	addPawn(1, 6, BLACK, c)
	addPawn(2, 6, BLACK, c)
	addPawn(3, 6, BLACK, c)
	addPawn(4, 6, BLACK, c)
	addPawn(5, 6, BLACK, c)
	addPawn(6, 6, BLACK, c)
	addPawn(7, 6, BLACK, c)
	// TODO: Other pieces
}

func clearBoard(c chan<- bop) {
	piecesc := make(chan piece)
	c <- bopGetAllPieces(piecesc)
	// Two-step to avoid dead-lock
	pieces := []piece{}
	for p := range piecesc {
		pieces = append(pieces, p)
	}
	for _, p := range pieces {
		c <- bopDelPiece(p)
	}
	return
}

func main() {
	boardc := make(chan bop)
	boarddone := make(chan bool)
	go runBoard(boardc, boarddone)
	initBoard(boardc)
	clearBoard(boardc)
}
