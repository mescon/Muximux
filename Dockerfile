FROM php:7.0-apache

RUN apt-get update && apt-get install -y git

RUN git clone https://github.com/mescon/Muximux /var/www/html

WORKDIR /var/www/html

EXPOSE 80
