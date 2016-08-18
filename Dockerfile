FROM php:7.0-apache

RUN git clone https://github.com/mescon/Muximux /var/www/html

WORKDIR /var/www/html

EXPOSE 80
