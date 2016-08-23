# Muximux - Lightweight HTPC portal

[![Join the chat at https://gitter.im/mescon/Muximux](https://badges.gitter.im/mescon/Muximux.svg)](https://gitter.im/mescon/Muximux?utm_source=badge&utm_medium=badge&utm_campaign=pr-badge&utm_content=badge)

> This is a PHP enabled fork of (the simpler and more lightweight) "Managethis" found here:
> https://github.com/Tenzinn3/Managethis

> If you prefer a NodeJS version with a built-in webserver, it is available at
> https://github.com/onedr0p/manage-this-node

This is a lightweight portal to view & manage your HTPC apps without having to run anything more than a PHP enabled webserver.
With Muximux you don't need to keep multiple tabs open, or bookmark the URL to all of your apps.

![Desktop screenshot](https://i.imgur.com/gqdVM6p.jpg)
[More screenshots](#screenshots)

## Major features
* Add, remove and rearrange your owns apps without touching any code - it's all in the settings menu!
* A shiny new dropdown menu (top right) where you can put items you don't use that often!
* Change or replace icons by just clicking the icon you think looks good.
* Enable or disable a landingpage for each app (landingpages prevent you from being bombarded with login-prompts, and reduces load on your browser).
* All menu items move to the dropdown when you access Muximux from your mobile phone or tablet!
* Refresh button - when you click it, only the app you are looking at will be reloaded - not EVERY app inside your browser. You can also double click the item in the menu.

### Behind the scenes features
* Deferred loading of apps - each app only opens when you first click it. Loading time of Muximux is very fast!
* Security token generated on each page load. To execute specific functions of Muximux you can not do it without this token - a token that changes when the user leaves the page, effectively making commands to Muximux not function if you are not a valid user of the Muximux app currently browsing it.
* API calls to Github to look up commit history/changelog are cached and only called *once* when Muximux is loaded.
* No HTTP requests to external servers. *Muximux fonts, icons and other resources: Google, Bootstrap, jQuery and Font-Awesome do not need to know you are hosting a server!*
* Custom versions of minified javascript libraries that removes some of the unnecessary functions we're not using, which result in less javascript overhead and faster loading times.

## Setup

**Requirements:** A webserver (nginx, Apache, IIS or any other webserver) configured with PHP5 support.
`` parse_ini_file `` must be allowed in php.ini (default is allowed!)

- To set it up, clone this repository:
`` git clone https://github.com/mescon/Muximux `` or [download the ZIP-file](https://github.com/mescon/Muximux/archive/master.zip). *(note: If you do not install via the git method, you will not be be able to compare your version with the latest update to Muximux)*

- Place all files on a publically accessible webserver, either in a subdirectory called ``muximux`` or directly in the root directory of your webserver (such as ``/var/www``, ``/var/html``, ``C:\Inetpub\wwwroot`` or wherever your webserver serves files from by default).

- [Read this note](#security) about securing Muximux, and [read this note](#important-note-regarding-https) about what happens if you are using HTTPS. Just do it.

- Make sure that the directory where you place Muximux is [writable by the process that is running your webserver](http://lmgtfy.com/?q=how+to+make+a+directory+writable+by+my+webserver). *(i.e www-data, www-user, apache, nginx or whatever the user is called)*
  - Example: ``chown -R www-data.www-data /var/www/muximux``

> **Users of Muximux versions prior to v1.0**
> *Users of Muximux 0.9.1 only need to overwrite with the new files - unfortunately, your config settings will not be transferred to Muximux v1.0. You can click "Show backup INI" under "Settings" to see the contents of your old config.*
> *There is no need to edit either config.ini.php or settings.ini.php - in fact, we recommend you don't!*
> *Your settings.ini.php will never be overwritten if you use ``git pull`` or download the ZIP-file again.*

## Docker Setup

1. Install [Docker](https://www.docker.com/)

2. Make directory to store Muximux config files. Navigate to that directory and download the sample config.
```bash
mkdir muximux
curl -O https://raw.githubusercontent.com/mescon/Muximux/master/settings.ini.php-example muximux/settings.ini.php
cd muximux
```
3. Run the container, pointing to the directory with the config file. This should now pull the image from Docker hub:
```bash
docker run -d -p 80:80 \
--name="muximux" \
-v $(pwd):/config \
--restart="always" \
mescon/muximux
```

### Config File
```
-v $(pwd):/config \
```
That will give the absoulte path to your muximux folder. It will be linked to your config in the contatiner so that if you need to rebuild the container you will retain your configuration.

### Port Conflicts
If you run into a port conflict trying to run on 80, it is simple to modify the port forwarding:

```bash
-p 81:80
```


## Usage
- Navigate to ``http://<host>/muximux`` where ``<host>`` is either the IP or hostname of your webserver. *Obviously if you put the files in the root directory of your webserver, there is no need to append ``/muximux``*

- Remove the default apps (or just change the URL:s of them if you want to keep them), add your own apps by clicking in the top right corner and then click "Settings".

- Under Settings, rearrange your apps with drag'n'drop - just drag an item under another item to move it it.

- To reload an app, double click it in the menu, or press the refresh button in the top right bar.

> There is no longer any need to edit config.ini.php or any file at all. In fact, we recommend you don't!

### Security
**It is strongly recommended that you secure Muximux with Basic Auth (``.htpasswd / .htaccess``)**

Read instructions for [Nginx](https://www.digitalocean.com/community/tutorials/how-to-set-up-password-authentication-with-nginx-on-ubuntu-14-04), [Apache](https://www.digitalocean.com/community/tutorials/how-to-set-up-password-authentication-with-apache-on-ubuntu-14-04) and [Microsoft IIS](http://serverfault.com/a/272292).

If you decide not to, Muximux disallows search engines from indexing your site, however, Muximux itself does not password protect your services, so you have to secure each of your applications properly (which they already should be!).
Muximux is NOT a proxy server, and as such can not by itself secure your separate applications, and if you don't want Muximux to be open to the world, you *must* make sure that you have Basic Auth enabled on your server.

### Important note regarding HTTPS
 If you are serving Muximux from a HTTPS-enabled webserver (i.e``https://myserver.com/muximux``), all of your services must also be secured via HTTPS.
 Any URL that does not start with https:// (such as ``http://myserver.com:5050``) will be blocked by your web-browser!

 If you can, try serving Muximux from a non-secure (HTTP) webserver instead.
 If the apps you have configured are using HTTPS, communication with them will still be encrypted.

 The only known workaround is for Chrome, Opera and Chromium-based webbrowsers.

 Install the plugin "[Ignore X-Frame headers](https://chrome.google.com/webstore/detail/ignore-x-frame-headers/gleekbfjekiniecknbkamfmkohkpodhe)" which disables the blocking of non-secure content.


## Screenshots
#### Desktop screenshot
![Desktop screenshot](https://i.imgur.com/gqdVM6p.jpg)

#### Mobile screenshot - dropdown menu hidden
![Mobile screenshot - dropdown menu hidden](https://i.imgur.com/w8WjHiO.jpg)

#### Mobile screenshot - dropdown menu shown
![Mobile screenshot - dropdown menu shown](https://i.imgur.com/vsVtrvG.jpg)

#### Settings: Drag & Drop items to re-arrange them in your menu
![Drag & Drop items to re-arrange them in your menu](https://i.imgur.com/JKZvn74.jpg)

#### Settings: Pick and choose from over 500 icons
![Pick and choose from over 500 icons](https://i.imgur.com/KsuOzH1.jpg)
