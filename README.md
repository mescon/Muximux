# Muximux - Lightweight HTPC portal

>This is a PHP enabled fork of (the simpler and more lightweight) "Managethis" found here:
> https://github.com/Tenzinn3/Managethis


> If you prefer a NodeJS version with a built-in webserver, it is available at https://github.com/onedr0p/manage-this-node

This is a lightweight portal to manage your HTPC apps without having to run anything more than a PHP enabled webserver.
With Muximux you don't need to keep multiple tabs open, or bookmark the URL to all of your apps.

**Desktop screenshot**

![Desktop screenshot](http://i.imgur.com/VguPTzR.jpg)

**Mobile screenshot**

![Mobile screenshot](https://i.imgur.com/a4N3TLx.jpg)

#### Added features and fixes
* No need to edit HTML pages anywhere.
* Everything is configured in an easy config-file!
  * *Just rename config.ini.php-example to config.ini.php and open it up in your favorite text editor!*
  * *Your config.ini.php will never be overwritten if you use ``git pull`` or download the ZIP-file again.*

* You now have the possibility to easily:
  * Enable or disable any app or site.
  * Enable or disable a landingpage for each app (landingpages prevent you from being bombarded with login-prompts, and reduces load on your browser).
  * Change or replace icons.
  * Add your own apps, without having to delete, change or extend any code - it's all in the configuration file!

* Added a "refresh" icon. When you click it, the app or site you are currently using will be reloaded - not every app you've configured, which is very useful if you're having temporary problems with one of your apps/sites and don't want to reload every single app you have configured.
  * *You can also double click on the item you want to refresh in the menu*

* Fixed an issue with Internet Explorer which would result in a scrollbar being present in the menu.
* Custom versions of minified javascript libraries that removes some of the unnecessary functions we're not using, which result in less javascript overhead and faster loading times.

* Moved everything CSS-related to a CSS-file (no inline CSS in the HTML)

* All the logic in a separate file called ``muximux.php`` - no need to touch it!

#### The future
* I'll be fiddling with this on and off, and when I haven't found any bugs myself, or had any bugs filed for a while I will release version 1.0. After that, I'm taking suggestions for new features! In the meantime I'm happy to accept any pull requests/merge requests.



## Setup
**Requirements:** A webserver (nginx, Apache, IIS or any other webserver) configured with PHP5 support.
`` parse_ini_file `` must be allowed in php.ini (default is allowed!)
- To set it up, clone this repository:
`` git clone https://github.com/mescon/Muximux `` or download the ZIP-file.
- Place all files on a publically accessible webserver, either directly in the root, or inside a directory called ``muximux`` or whatever you want it to be called.
- Rename ``config.ini.php-example`` to ``config.ini.php`` *(Note: Your ``config.ini.php`` will never be overwritten if you update to a new version)*
- In your favourite text-editor open ``config.ini.php`` and read the instructions.
  - You can enable or disable apps simply by setting ``enabled = "true"`` or ``enabled = "false"``
  - You can change the app icons by replacing them with ones from http://bootstrapdocs.com/v3.0.0/docs/components/ or http://fontawesome.io/icons/
- The configuration file ``config.ini.php`` can not be read by any visitor, as long as you don't remove the top part of the file.

 > **Important note regarding HTTPS:**
 > If you are serving Muximux from a HTTPS-enabled webserver (i.e``https://myserver.com/muximux``), all of your services must also be secured via HTTPS.
 > Any URL that does not start with https:// (such as ``http://myserver.com:5050``) will be blocked by your web-browser!
 >
 > If you can, try serving Muximux from a non-secure (HTTP) webserver instead.
 >
 > The only known workaround is for Chrome, Opera and Chromium-based webbrowsers.
 > The plugin "Ignore X-Frame headers" disables the blocking of non-secure content.
 > https://chrome.google.com/webstore/detail/ignore-x-frame-headers/gleekbfjekiniecknbkamfmkohkpodhe

## Usage
- Navigate to ``http://<host>/muximux`` where ``<host>`` is either the IP or hostname of your webserver. *Obviously if you put the files in the root directory of your webserver, there is no need to append ``/muximux``*
- Access your apps by clicking on the "Launch" button. If you don't see a "Launch" button, you have ``landingpage = "false"`` configured for the app you're linking to. *(Note: This functionality was implemented to stop you from being hit by multiple login popups as soon as you start the app. It also speeds up loading time.)*
- To reload an app, double click it in the menu, or press the refresh button in the top right bar.

### Notes
**It is strongly recommended that you secure Muximux with Basic Auth (``.htpasswd / .htaccess``)**
Instructions for [Nginx](https://www.digitalocean.com/community/tutorials/how-to-set-up-password-authentication-with-nginx-on-ubuntu-14-04), [Apache](https://www.digitalocean.com/community/tutorials/how-to-set-up-password-authentication-with-apache-on-ubuntu-14-04) and [Microsoft IIS](http://serverfault.com/a/272292).
If you decide not to, Muximux disallows search engines from indexing your site, however, Muximux itself does not password protect your services, so you have to secure each of your applications properly (which they already should be!).
