#!/bin/bash
# اسکریپت برای نصب پیش‌نیازهای توسعه و اجرای اولیه روی اوبونتو 22.04

echo "شروع نصب پیش‌نیازها..."

# بروزرسانی لیست پکیج‌ها
sudo apt update && sudo apt upgrade -y

# نصب ابزارهای ضروری
echo "در حال نصب ابزارهای ضروری (curl, wget, git, build-essential)..."
sudo apt install -y curl wget git build-essential

# نصب Go (آخرین نسخه)
echo "در حال نصب Go..."
GO_VERSION_INFO=$(curl -s "https://go.dev/dl/?mode=json" | grep -oP '"version":\s*"\Kgo[0-9\.]+(?=")')
if [ -z "$GO_VERSION_INFO" ]; then
    echo "خطا: اطلاعات نسخه Go دریافت نشد. لطفاً اتصال اینترنت و آدرس https://go.dev/dl/?mode=json را بررسی کنید."
    # استفاده از یک نسخه پیش‌فرض در صورت عدم دریافت اطلاعات
    GO_VERSION_INFO="go1.21.5" # می‌توانید این نسخه را به‌روز نگه دارید
    echo "استفاده از نسخه پیش‌فرض Go: $GO_VERSION_INFO"
fi
GO_TARBALL="${GO_VERSION_INFO}.linux-amd64.tar.gz"
DOWNLOAD_URL="https://dl.google.com/go/${GO_TARBALL}"

echo "درحال دانلود ${DOWNLOAD_URL}..."
wget "${DOWNLOAD_URL}" -O /tmp/${GO_TARBALL}

if [ $? -ne 0 ]; then
    echo "خطا در دانلود Go. لطفاً اتصال اینترنت و URL دانلود را بررسی کنید."
    exit 1
fi

sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf /tmp/${GO_TARBALL}
rm /tmp/${GO_TARBALL}

# افزودن Go به PATH برای همه کاربران (نیاز به logout/login یا source کردن پروفایل)
echo "پیکربندی متغیرهای محیطی Go..."
if ! grep -q "/usr/local/go/bin" /etc/profile.d/go.sh 2>/dev/null; then
    echo 'export PATH=$PATH:/usr/local/go/bin' | sudo tee /etc/profile.d/go.sh
    echo 'export GOPATH=$HOME/go' | sudo tee -a /etc/profile.d/go.sh
    echo 'export PATH=$PATH:$GOPATH/bin' | sudo tee -a /etc/profile.d/go.sh
    # ایجاد دایرکتوری‌های GOPATH
    mkdir -p $HOME/go/src $HOME/go/bin $HOME/go/pkg
    sudo chmod -R 777 $HOME/go # اطمینان از دسترسی نوشتن
else
    echo "متغیرهای محیطی Go قبلاً تنظیم شده‌اند یا فایل go.sh موجود است."
fi


# اعمال تغییرات PATH در session فعلی (برای ادامه اسکریپت اگر نیاز باشد)
export PATH=$PATH:/usr/local/go/bin
# GOROOT معمولا توسط خود Go مدیریت می‌شود. GOPATH برای پروژه‌های قدیمی‌تر است اما داشتنش ضرری ندارد.
export GOPATH=$HOME/go 
mkdir -p $GOPATH/src $GOPATH/bin $GOPATH/pkg
export PATH=$PATH:$GOPATH/bin


echo "Go $(go version) نصب شد."

# نصب Docker
echo "در حال نصب Docker..."
# حذف نسخه‌های قدیمی Docker (اگر وجود داشته باشد)
sudo apt-get remove docker docker-engine docker.io containerd runc -y 2>/dev/null || true
# تنظیم ریپازیتوری Docker
sudo apt-get install -y apt-transport-https ca-certificates curl gnupg lsb-release
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo gpg --dearmor -o /usr/share/keyrings/docker-archive-keyring.gpg
echo \
  "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/docker-archive-keyring.gpg] https://download.docker.com/linux/ubuntu \
  $(lsb_release -cs) stable" | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null
sudo apt-get update
# نصب Docker Engine, CLI, و Containerd
sudo apt-get install -y docker-ce docker-ce-cli containerd.io

# افزودن کاربر فعلی به گروه docker (برای اجرای دستورات docker بدون sudo)
# نیاز به logout/login دارد تا اعمال شود
if ! getent group docker | grep -q "\b${USER}\b"; then
    sudo usermod -aG docker ${USER}
    echo "کاربر ${USER} به گروه docker اضافه شد."
    NEEDS_RELOGIN=true
else
    echo "کاربر ${USER} از قبل عضو گروه docker است."
    NEEDS_RELOGIN=false
fi


echo "Docker نصب شد."

# نصب Docker Compose V2 (به عنوان پلاگین داکر)
echo "در حال نصب Docker Compose..."
sudo apt-get install -y docker-compose-plugin
# بررسی نصب docker compose
docker compose version
if [ $? -ne 0 ]; then
    echo "هشدار: به نظر می‌رسد docker-compose-plugin به درستی نصب نشده یا دستور 'docker compose' کار نمی‌کند."
    echo "تلاش برای نصب Docker Compose از طریق GitHub release..."
    COMPOSE_VERSION=$(curl -s https://api.github.com/repos/docker/compose/releases/latest | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    if [ -z "$COMPOSE_VERSION" ]; then
        COMPOSE_VERSION="v2.23.0" # یک نسخه پایدار در صورت عدم دریافت از API
    fi
    echo "نصب Docker Compose نسخه ${COMPOSE_VERSION}..."
    DOCKER_CONFIG=${DOCKER_CONFIG:-$HOME/.docker}
    mkdir -p $DOCKER_CONFIG/cli-plugins
    curl -SL https://github.com/docker/compose/releases/download/${COMPOSE_VERSION}/docker-compose-linux-$(uname -m) -o $DOCKER_CONFIG/cli-plugins/docker-compose
    chmod +x $DOCKER_CONFIG/cli-plugins/docker-compose
    docker compose version
    if [ $? -ne 0 ]; then
        echo "خطا: نصب Docker Compose از GitHub نیز موفقیت‌آمیز نبود."
    else
        echo "Docker Compose با موفقیت از GitHub نصب شد."
    fi
fi

echo ""
echo "---------------------------------------------------------------------"
echo "نصب پیش‌نیازهای پایه کامل شد."
echo "---------------------------------------------------------------------"
echo ""
echo "نسخه‌های نصب شده:"
go version
docker --version
docker compose version || echo "Docker Compose ممکن است به درستی نصب نشده باشد."
git --version
echo ""
echo "توجهات مهم:"
if [ "$NEEDS_RELOGIN" = true ]; then
    echo "  - لطفاً یکبار از سیستم خارج (logout) و دوباره وارد (login) شوید، یا یک session جدید ترمینال باز کنید،"
    echo "    تا بتوانید دستورات docker را بدون نیاز به sudo اجرا نمایید."
    echo "  - یا می‌توانید در همین ترمینال دستور 'newgrp docker' را اجرا کنید (ممکن است نیاز به رمز عبور داشته باشد)."
fi
echo "  - متغیرهای محیطی Go در /etc/profile.d/go.sh تنظیم شده‌اند و در session بعدی ترمینال اعمال خواهند شد."
echo "    برای اعمال فوری در همین ترمینال، می‌توانید دستور 'source /etc/profile.d/go.sh' را اجرا کنید."
echo "    و همچنین 'export GOPATH=\$HOME/go; mkdir -p \$GOPATH/src \$GOPATH/bin \$GOPATH/pkg; export PATH=\$PATH:\$GOPATH/bin' "

echo ""
echo "اسکریپت به پایان رسید."
