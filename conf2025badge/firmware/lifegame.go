// https://github.com/akinobufujii/lifegame_tinygo/blob/master/lifegame.go

package main

import (
	"machine"
	"math/rand/v2"
)

const MaxGeneration int = 2

type Cells [][]bool

type LifeGame struct {
	width, height          int
	cells                  [MaxGeneration]Cells // 現世代/次世代
	currentGenerationIndex int
	count                  int
}

func NewLifeGame(width, height int) (*LifeGame, error) {
	lifeGame := &LifeGame{width: width, height: height}

	for i := range lifeGame.cells {
		lifeGame.cells[i] = make([][]bool, height)

		for y := range lifeGame.cells[i] {
			lifeGame.cells[i][y] = make([]bool, width)
		}
	}

	return lifeGame, nil
}

func (p *LifeGame) Update() {
	currentCells := p.GetCurrentGenerationCells()
	nextGenerationIndex := (p.currentGenerationIndex + 1) % MaxGeneration
	nextCells := p.cells[nextGenerationIndex]

	for y := range currentCells {
		for x := range currentCells[y] {
			p.GetAround(x, y) // update dead and alive
			deadOrAlive := currentCells[y][x]
			if deadOrAlive {
				if alive == 2 || alive == 3 {
					// 生存：生きているセルに隣接する生きたセルが2つか3つならば、次の世代でも生存する。
					nextCells[y][x] = true
				} else if alive <= 1 {
					// 過疎：生きているセルに隣接する生きたセルが1つ以下ならば、過疎により死滅する。
					nextCells[y][x] = false
				} else if alive >= 4 {
					// 過密：生きているセルに隣接する生きたセルが4つ以上ならば、過密により死滅する。
					nextCells[y][x] = false
				}
			} else {
				if alive == 3 {
					// 誕生：死んでいるセルに隣接する生きたセルがちょうど3つあれば、次の世代が誕生する。
					nextCells[y][x] = true
				} else {
					nextCells[y][x] = false
				}
			}
		}
	}

	p.currentGenerationIndex = nextGenerationIndex

	p.count++
	if p.count > 200 {
		p.count = 0
		p.InitRandom()
	}
}

func (p *LifeGame) InitRandom() {
	seed1, _ := machine.GetRNG()
	seed2, _ := machine.GetRNG()
	source := rand.NewPCG(uint64(seed1), uint64(seed2))
	r := rand.New(source)

	cells := p.cells[p.currentGenerationIndex]

	for y := range cells {
		for x := range cells[y] {
			// NOTE: 初期の生存率を下げるための適当な初期値を代入
			value := r.IntN(100) % 5
			cells[y][x] = value == 0
		}
	}
}

func (p *LifeGame) GetCells() Cells {
	return p.cells[p.currentGenerationIndex]
}

var (
	// for GetAround()
	xIndex int
	yIndex int
	dead   int
	alive  int
)

func (p *LifeGame) GetAround(x, y int) {
	dead = 0
	alive = 0
	// NOTE: 端の判定は上下左右がつながってることにする
	cells := p.GetCurrentGenerationCells()

	xIndex, yIndex = 0, 0
	calcDeadOrAlive := func() {
		if cells[yIndex][xIndex] {
			alive++
		} else {
			dead++
		}
	}

	// 上
	xIndex = x
	yIndex = (y - 1)
	if yIndex < 0 {
		yIndex = p.height - 1
	}
	calcDeadOrAlive()

	// 下
	xIndex = x
	yIndex = (y + 1) % p.height
	calcDeadOrAlive()

	// 左
	xIndex = x - 1
	if xIndex < 0 {
		xIndex = p.width - 1
	}
	yIndex = y
	calcDeadOrAlive()

	// 右
	xIndex = (x + 1) % p.width
	yIndex = y
	calcDeadOrAlive()

	// 右上
	xIndex = (x + 1) % p.width
	yIndex = (y - 1)
	if yIndex < 0 {
		yIndex = p.height - 1
	}
	calcDeadOrAlive()

	// 左上
	xIndex = x - 1
	if xIndex < 0 {
		xIndex = p.width - 1
	}
	yIndex = (y - 1)
	if yIndex < 0 {
		yIndex = p.height - 1
	}
	calcDeadOrAlive()

	// 右下
	xIndex = (x + 1) % p.width
	yIndex = (y + 1) % p.height
	calcDeadOrAlive()

	// 左下
	xIndex = x - 1
	if xIndex < 0 {
		xIndex = p.width - 1
	}
	yIndex = (y + 1) % p.height
	calcDeadOrAlive()

	return
}

func (p *LifeGame) GetCurrentGenerationCells() Cells {
	return p.cells[p.currentGenerationIndex]
}
