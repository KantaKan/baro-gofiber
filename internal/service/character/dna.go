package character

import (
	"crypto/rand"
	"errors"
	"io"
	"math/big"

	"gofiber-baro/internal/domain"
)

type Picker interface {
	Intn(limit int) (int, error)
}

type SecurePicker struct {
	Reader io.Reader
}

func (picker SecurePicker) Intn(limit int) (int, error) {
	if limit <= 0 {
		return 0, errors.New("random limit must be positive")
	}
	reader := picker.Reader
	if reader == nil {
		reader = rand.Reader
	}
	value, err := rand.Int(reader, big.NewInt(int64(limit)))
	if err != nil {
		return 0, err
	}
	return int(value.Int64()), nil
}

var bodies = []string{"pebble", "bean", "drop", "tall", "pillow", "pear", "squish", "cloud", "boxy", "wobble", "mushroom", "dumpling"}
var ears = []string{"round", "point", "horn", "antenna", "leaf", "cat", "none"}
var eyes = []string{"dots", "oval", "sleepy", "spark", "wide"}
var marks = []string{"star", "spots", "stripe", "heart", "moon", "none"}
var palettes = []string{"Mint", "Peach", "Lilac", "Honey", "Lagoon", "Berry", "Moss", "Cloud"}
var commonPatterns = []string{"freckles", "polka", "stripes", "waves", "marble", "checker", "sprouts", "hearts", "bubbles", "zigzag", "mosaic", "constellation", "paint", "petals"}
var rarePatterns = []string{"egg", "potato", "ramen", "error404"}

func RarityForRoll(roll int) (domain.CharacterRarity, error) {
	if roll < 0 || roll >= 10000 {
		return "", errors.New("rarity roll must be between 0 and 9999")
	}
	if roll < 8300 {
		return domain.CharacterNormal, nil
	}
	if roll < 9800 {
		return domain.CharacterMemeRare, nil
	}
	return domain.CharacterLegendary, nil
}

func GenerateDNA(picker Picker) (domain.CharacterDNA, error) {
	if picker == nil {
		return domain.CharacterDNA{}, errors.New("random picker is required")
	}
	roll, err := picker.Intn(10000)
	if err != nil {
		return domain.CharacterDNA{}, err
	}
	rarity, err := RarityForRoll(roll)
	if err != nil {
		return domain.CharacterDNA{}, err
	}
	patternPool := commonPatterns
	if rarity != domain.CharacterNormal {
		patternPool = rarePatterns
	}
	values := make([]string, 0, 6)
	for _, pool := range [][]string{bodies, ears, eyes, marks, palettes, patternPool} {
		index, pickErr := picker.Intn(len(pool))
		if pickErr != nil {
			return domain.CharacterDNA{}, pickErr
		}
		if index < 0 || index >= len(pool) {
			return domain.CharacterDNA{}, errors.New("random picker returned an invalid index")
		}
		values = append(values, pool[index])
	}
	patternSeed, err := picker.Intn(1000000)
	if err != nil {
		return domain.CharacterDNA{}, err
	}
	if patternSeed < 0 || patternSeed >= 1000000 {
		return domain.CharacterDNA{}, errors.New("random picker returned an invalid pattern seed")
	}
	return domain.CharacterDNA{
		Version: 1, Body: values[0], Ears: values[1], Eyes: values[2], Mark: values[3],
		Palette: values[4], Pattern: values[5], PatternSeed: uint32(patternSeed), Rarity: rarity,
	}, nil
}
