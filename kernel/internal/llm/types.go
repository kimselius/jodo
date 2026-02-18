package llm

// EmbedResponse holds the result of an embedding request.
type EmbedResponse struct {
	Embedding []float32
	Model     string
	Provider  string
	TokensIn  int
	Cost      float64
}

// BudgetStatus holds the budget info for a single provider.
type BudgetStatus struct {
	MonthlyBudget      float64 `json:"monthly_budget"`
	SpentThisMonth     float64 `json:"spent_this_month"`
	Remaining          float64 `json:"remaining"`
	EmergencyReserve   float64 `json:"emergency_reserve"`
	AvailableForNormal float64 `json:"available_for_normal_use"`
}
