FROM php:7.0-apache

RUN apt-get update && apt-get install -y git && \
git clone https://github.com/mescon/Muximux /var/www/html  && \

# cleanup
apt-get clean && rm -rf /tmp/* /var/lib/apt/lists/* /var/tmp/* 

WORKDIR /var/www/html

VOLUME /config

RUN ln -s /config/settings.ini.php /var/www/html/settings.ini.php

EXPOSE 80







