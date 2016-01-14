# MTPHP
A lightweight way to manage your HTPC, dynamically handled with PHP and a simple configuration-file.

This is a PHP enabled fork of (the simpler and more lightweight) "Managethis" found here: https://github.com/Tenzinn3/Managethis

This is a lightweight way to manage your HTPC apps without having to run anything extra, all you need is to have a webserver. It basically acts as a portal for all of your apps in once place so you don't need to keep multiple tabs open.

![alt tag](http://i.imgur.com/04Y0tDD.jpg)

## Setup

- To set it up clone and place the folder inside the root directory of your webserver then rename config.inc.php-example to config.inc.php
- In your favourite editor open config.inc.php and read the instructions.
- You can change the app icons by replacing them with ones from http://bootstrapdocs.com/v3.0.0/docs/components/ or http://fontawesome.io/icons/
- Navigate to http://youripaddress/mtphp to access MTPHP.
- You can access your apps by clicking on the "Launch" button. This was implemented to stop you being hit by multiple login requests as soon as you start the app. It also speeds up loading time. You can enable or disable this functionality in the configuration file, for each app.
- To reload an app, double click it in the menu. Only that specific page will reload.
- You can enable or disable specific apps without having to remove them entirely by just editing the configuration file.
- The configuration by default can not be read by a visitor, as long as you don't remove the top part of the file.

You may want to setup htaccess to secure it but even if you don't your apps will not be accessible as long as they themselves are secure.
