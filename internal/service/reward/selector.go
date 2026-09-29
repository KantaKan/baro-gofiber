package reward

import (
	"errors"
	"math"
	"math/rand/v2"

	"gofiber-baro/internal/domain"
)

var ErrPoolComplete = errors.New("reward pool has no eligible unowned items")
var ErrInvalidRandomValue = errors.New("random source must return a value from 0 up to but not including 1")

var rarityOrder = []string{"Common", "Rare", "Epic", "Legendary"}
var standardWeights = map[string]float64{"Common": 55, "Rare": 30, "Epic": 12, "Legendary": 3}

type RandomSource interface {
	Float64() float64
}

type SystemRandom struct{}

func (SystemRandom) Float64() float64 {
	return rand.Float64()
}

type SelectionInput struct {
	Catalog       []domain.CosmeticCatalogItem
	Pool          string
	MinimumRarity string
	Species       string
	OwnedIDs      map[string]bool
}

type Selector struct {
	random RandomSource
}

type Eligibility struct {
	EligibleCount int                `json:"eligible_count"`
	Odds          map[string]float64 `json:"odds"`
	Complete      bool               `json:"complete"`
}

func NewSelector(random RandomSource) *Selector {
	return &Selector{random: random}
}

func DisclosedOdds(minimumRarity string) map[string]float64 {
	floor := rarityIndex(minimumRarity)
	if floor < 0 {
		floor = 0
	}
	total := 0.0
	for index, rarity := range rarityOrder {
		if index >= floor {
			total += standardWeights[rarity]
		}
	}
	odds := map[string]float64{}
	for index, rarity := range rarityOrder {
		if index >= floor {
			odds[rarity] = standardWeights[rarity] / total
		}
	}
	return odds
}

func (s *Selector) Select(input SelectionInput) (domain.CosmeticCatalogItem, error) {
	candidates := eligibleCandidates(input)
	if len(candidates) == 0 {
		return domain.CosmeticCatalogItem{}, ErrPoolComplete
	}

	total := 0.0
	for rarity := range candidates {
		total += standardWeights[rarity]
	}
	rarityRoll, err := draw(s.random)
	if err != nil {
		return domain.CosmeticCatalogItem{}, err
	}
	threshold := rarityRoll * total
	chosenRarity := ""
	cumulative := 0.0
	for _, rarity := range rarityOrder {
		if len(candidates[rarity]) == 0 {
			continue
		}
		cumulative += standardWeights[rarity]
		if threshold < cumulative {
			chosenRarity = rarity
			break
		}
	}
	if chosenRarity == "" {
		for index := len(rarityOrder) - 1; index >= 0; index-- {
			if len(candidates[rarityOrder[index]]) > 0 {
				chosenRarity = rarityOrder[index]
				break
			}
		}
	}

	items := candidates[chosenRarity]
	itemRoll, err := draw(s.random)
	if err != nil {
		return domain.CosmeticCatalogItem{}, err
	}
	index := int(math.Floor(itemRoll * float64(len(items))))
	return items[index], nil
}

func (s *Selector) Eligibility(input SelectionInput) Eligibility {
	candidates := eligibleCandidates(input)
	result := Eligibility{Odds: map[string]float64{}, Complete: len(candidates) == 0}
	totalWeight := 0.0
	for rarity, items := range candidates {
		result.EligibleCount += len(items)
		totalWeight += standardWeights[rarity]
	}
	for rarity := range candidates {
		result.Odds[rarity] = standardWeights[rarity] / totalWeight
	}
	return result
}

func eligibleCandidates(input SelectionInput) map[string][]domain.CosmeticCatalogItem {
	candidates := map[string][]domain.CosmeticCatalogItem{}
	floor := rarityIndex(input.MinimumRarity)
	if floor < 0 {
		floor = 0
	}
	for _, item := range input.Catalog {
		if item.Starter || input.OwnedIDs[item.ID] || rarityIndex(item.Rarity) < floor || rarityIndex(item.Rarity) < 0 || !inPool(item, input.Pool) || !compatible(item, input.Species) {
			continue
		}
		candidates[item.Rarity] = append(candidates[item.Rarity], item)
	}
	return candidates
}

func draw(random RandomSource) (float64, error) {
	value := random.Float64()
	if value < 0 || value >= 1 || math.IsNaN(value) {
		return 0, ErrInvalidRandomValue
	}
	return value, nil
}

func rarityIndex(rarity string) int {
	for index, candidate := range rarityOrder {
		if candidate == rarity {
			return index
		}
	}
	return -1
}

func inPool(item domain.CosmeticCatalogItem, pool string) bool {
	for _, candidate := range item.RewardPools {
		if candidate == pool {
			return true
		}
	}
	return false
}

func compatible(item domain.CosmeticCatalogItem, species string) bool {
	if len(item.CompatibleSpecies) == 0 {
		return true
	}
	for _, candidate := range item.CompatibleSpecies {
		if candidate == species {
			return true
		}
	}
	return false
}
