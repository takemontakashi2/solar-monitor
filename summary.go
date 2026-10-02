package main

func deriveECHONETSummary(sample *Sample) {
	sample.PVWatts = nil
	sample.LoadWatts = nil
	sample.GridWatts = nil
	sample.TodayKWh = nil
	for _, prop := range sample.Properties {
		if prop.Float == nil {
			continue
		}
		switch prop.EOJ + ":" + prop.EPC {
		case "027901:e0":
			sample.PVWatts = floatPtr(*prop.Float)
		case "027901:e1":
			sample.TodayKWh = floatPtr(*prop.Float * 0.001)
		}
	}
}
