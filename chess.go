package main

import (
	"fmt"
)

type coords struct {
	x, y int
}

type pieceType int

const (
	PAWN pieceType = iota
	KNIGHT
	BISHOP
	ROOK
	QUEEN
	KING
)

func (pt pieceType) String() string {
	switch pt {
	case PAWN:
		return "pawn"
	case KNIGHT:
		return "knight"
	case BISHOP:
		return "bishop"
	case ROOK:
		return "rook"
	case QUEEN:
		return "queen"
	case KING:
		return "king"
	}
	panic("Illegal piece type")
}

type op interface{}

type opGetCoords chan<- coords

type opSetCoords coords

// Die. Close this channel when operation acknowledged (for sync)
type opKill chan<- bool

// Subscribe to moves by request all coordinates updates be sent down here.
// Send nil channel to cancel.
type opMoveCallback chan<- coords

type opSetPieceType pieceType

type opGetPieceType chan<- pieceType

type pieceLoc struct {
	coords
	ctrl chan<- op
}

// Control operations are read from the control channel.
func spawnPiece(c <-chan op) {
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
		case opSetCoords:
			x = t.x
			y = t.y
			if movechan != nil {
				movechan <- coords(t)
			}
		case opGetCoords:
			t <- coords{x, y}
			close(t)
		case opKill:
			close(t)
			return
		case opMoveCallback:
			movechan = t
		case opSetPieceType:
			pt = pieceType(t)
		case opGetPieceType:
			t <- pt
			close(t)
		default:
			panic(fmt.Sprintf("Illegal operation: %v", op))
		}
	}
}

func addPawn(x, y int, mu chan<- op) chan<- op {
	c := make(chan op)
	// Start a block
	go spawnPiece(c)
	// piece will push updates to coordinates down this channel
	coordUpdates := make(chan coords)
	c <- opMoveCallback(coordUpdates)
	// Translate those updates to a message that includes the control channel
	go func() {
		for xy := range coordUpdates {
			mu <- pieceLoc{xy, c}
		}
	}()
	// Make it a pawn
	c <- opSetPieceType(PAWN)
	// Move it to the desired coordinates
	c <- opSetCoords{x, y}
	return c
}

// Run a board management unit. Push all location changes down this channel.
// Closes the done channel when all updates have been consumed and the input
// channel is closed (for sync).
func runBoard(c <-chan op, done chan<- bool) {
	for o := range c {
		switch t := o.(type) {
		case pieceLoc:
			pc := make(chan pieceType)
			t.ctrl <- opGetPieceType(pc)
			fmt.Printf("Moved %s to (%d, %d)\n", <-pc, t.x, t.y)
			break
		default:
			panic(fmt.Sprintf("Illegal board operation: %v", o))
		}
	}
	close(done)
}

// Initialize an empty chess board by putting pieces in the right places
func initBoard(c chan<- op) {
	addPawn(0, 1, c)
	addPawn(1, 1, c)
	addPawn(2, 1, c)
	addPawn(3, 1, c)
	addPawn(4, 1, c)
	addPawn(5, 1, c)
	addPawn(6, 1, c)
	addPawn(7, 1, c)
	// TODO: Other pieces
	// TODO: adversary
}

func main() {
	boardc := make(chan op)
	boarddone := make(chan bool)
	go runBoard(boardc, boarddone)
	initBoard(boardc)
	// TODO: clearBoard()...
}
