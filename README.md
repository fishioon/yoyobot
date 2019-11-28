## build
make

## run
./yoyobot -config config.json

## 导入数据
```
cd configs
curl -X POST http://127.0.0.1:8080/admin/set -d @day.json --header "Content-Type: application/json"
```
