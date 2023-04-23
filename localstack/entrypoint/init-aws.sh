#! /bin/bash

export AWS_ACCESS_KEY_ID="test"
export AWS_SECRET_ACCESS_KEY="test"

# terraform state bucket
awslocal s3 mb s3://local-terraform
