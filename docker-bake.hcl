group "default" {
  targets = ["test"]
}

target "test" {
  target = "test"
  output = ["type=cacheonly"]
}

