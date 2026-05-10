INSTALAÇÂO:
sudo pacman -S --needed poppler go nodejs npm
cd etl
go mod download
cd ../web
npm install

RODAR:
npm --prefix web run build && ./run.sh



ENVIAR DOCKER HUB:
sudo pacman -S docker docker-compose docker-buildx
sudo systemctl enable --now docker
docker-compose up --build

docker login
./deploy.sh v1.2.0 1

documentação em web/public/help/README.md
