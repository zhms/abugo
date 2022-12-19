@echo off
git add *
git commit -m'auto'
git push
git tag %1
git push --tags
