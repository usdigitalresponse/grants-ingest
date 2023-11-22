terraform {
  required_version = "1.5.1"
  required_providers {
    archive = {
      source  = "hashicorp/archive"
      version = "2.4.0"
    }
    external = {
      source  = "hashicorp/external"
      version = "2.3.2"
    }
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.23.1"
    }
  }
}
