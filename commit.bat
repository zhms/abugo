@echo off
git add *
git commit -m'auto'
git push
git tag v1.0.5
git push --tags
pause