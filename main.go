package main

func main() {
	 v, _ := newValidator( "http://0.0.0.0:9009/api/prom/api/v1/")
	 v.validate("sample.txt")
}
