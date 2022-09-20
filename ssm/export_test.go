package ssm

func MockNew(ssm ssmiface) *App {
	return &App{ssm: ssm}
}
