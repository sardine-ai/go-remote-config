package server

//func TestNewServer(t *testing.T) {
//	urlParsed, err := url.Parse("https://raw.githubusercontent.com/divakarmanoj/go-remote-config/go-only/test.yaml")
//	if err != nil {
//		t.Errorf("Error parsing url: %s", err.Error())
//	}
//	gitUrlParsed, err := url.Parse("https://github.com/divakarmanoj/go-remote-config.git")
//	if err != nil {
//		t.Errorf("Error parsing url: %s", err.Error())
//	}
//	Repositories := []source.Repository{
//		&source.FileRepository{Name: "file", Path: "../test.yaml"},
//		&source.WebRepository{URL: urlParsed, Name: "web"},
//		&source.GitRepository{URL: gitUrlParsed, Path: "test.yaml", Branch: "go-only", Name: "git"},
//	}
//	server := NewServer(context.Background(), Repositories, 10*time.Second)
//	server.Start("127.0.0.1:8076")

//// Test endpoints
//output, err := http.Get("http://127.0.0.1:8076/file")
//if err != nil {
//	t.Errorf("Error getting file: %s", err.Error())
//}
//if output.StatusCode != 200 {
//	t.Errorf("Expected status code to be 200, got %d", output.StatusCode)
//}
//
//output, err = http.Get("http://127.0.0.1:8076/web")
//if err != nil {
//	t.Errorf("Error getting web: %s", err.Error())
//}
//if output.StatusCode != 200 {
//	t.Errorf("Expected status code to be 200, got %d", output.StatusCode)
//}
//
//output, err = http.Get("http://127.0.0.1:8076/git")
//if err != nil {
//	t.Errorf("Error getting git: %s", err.Error())
//}
//if output.StatusCode != 200 {
//	t.Errorf("Expected status code to be 200, got %d", output.StatusCode)
//}
//server.Stop()
//os.Exit(0)
//}
