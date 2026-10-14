package main

func deriveECHONETSummary(sample *Sample) {
	sample.PVWatts = nil
	sample.LoadWatts = nil
	sample.GridWatts = nil
	sample.TodayKWh = nil
	var pvWatts, batteryWatts, gridSellWatts, gridBuyWatts *float64
	for _, prop := range sample.Properties {
		if prop.Float == nil {
			continue
		}
		switch prop.EOJ + ":" + prop.EPC {
		case "027901:e0":
			pvWatts = floatPtr(*prop.Float)
		case "027901:e1":
			sample.TodayKWh = floatPtr(*prop.Float * 0.001)
		case "027d01:d3":
			batteryWatts = floatPtr(*prop.Float)
		case "05ff01:f2":
			gridSellWatts = floatPtr(*prop.Float)
		case "05ff01:f3":
			gridBuyWatts = floatPtr(*prop.Float)
		}
	}
	sample.PVWatts = pvWatts
	if gridBuyWatts != nil && *gridBuyWatts > 0 {
		sample.GridWatts = gridBuyWatts
	} else if gridSellWatts != nil && *gridSellWatts > 0 {
		sample.GridWatts = floatPtr(-*gridSellWatts)
	} else if gridBuyWatts != nil || gridSellWatts != nil {
		sample.GridWatts = floatPtr(0)
	}
	if pvWatts != nil && batteryWatts != nil && gridSellWatts != nil && gridBuyWatts != nil {
		sample.LoadWatts = floatPtr(*pvWatts + *gridBuyWatts - *batteryWatts - *gridSellWatts)
	}
}
