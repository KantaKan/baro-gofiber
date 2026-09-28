package domain

type CosmeticCatalogItem struct {
	ID                string   `bson:"id" json:"id"`
	Name              string   `bson:"name" json:"name"`
	Slot              string   `bson:"slot" json:"slot"`
	Rarity            string   `bson:"rarity" json:"rarity"`
	PreviewValue      string   `bson:"preview_value" json:"preview_value"`
	SourceHint        string   `bson:"source_hint" json:"source_hint"`
	RewardPools       []string `bson:"reward_pools" json:"reward_pools"`
	Starter           bool     `bson:"starter" json:"starter"`
	CompatibleSpecies []string `bson:"compatible_species,omitempty" json:"compatible_species,omitempty"`
}

type CosmeticCollectionItem struct {
	CosmeticCatalogItem `bson:",inline"`
	Owned               bool `json:"owned"`
	New                 bool `json:"new"`
	Equipped            bool `json:"equipped"`
	Locked              bool `json:"locked"`
}

type CosmeticCollection struct {
	Items []CosmeticCollectionItem `json:"items"`
}
