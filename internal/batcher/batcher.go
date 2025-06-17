package batcher

// CreateBatches splits parameters into chunks of specified size
func CreateBatches(parameters []string, batchSize int) [][]string {
	if batchSize <= 0 {
		return [][]string{parameters}
	}

	var batches [][]string
	for i := 0; i < len(parameters); i += batchSize {
		end := i + batchSize
		if end > len(parameters) {
			end = len(parameters)
		}
		batches = append(batches, parameters[i:end])
	}

	return batches
}
