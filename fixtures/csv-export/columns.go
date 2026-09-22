package csvexport

var DefaultColumns = []string{"order_id", "customer", "paid_at"}

func ColumnCount() int { return len(DefaultColumns) }
