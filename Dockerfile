FROM php:7.0-apache

RUN apt-get update && apt-get install -y git && \
git clone https://github.com/mescon/Muximux /var/www/html  && \

# cleanup
apt-get clean && rm -rf /tmp/* /var/lib/apt/lists/* /var/tmp/* 


WORKDIR /var/www/html

EXPOSE 80
VOLUME /config
RUN ln -sf /config/settings.ini.php /var/www/html/settings.ini.php






