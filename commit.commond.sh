#!/bin/bash
cd `dirname $0`
git add *
git commit -m'auto'
git push
git tag %1
git push --tags
