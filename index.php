<?php
require 'muximux.php';
?><!doctype html>
<!--[if lt IE 7]>
<html class="no-js lt-ie9 lt-ie8 lt-ie7" lang="en"> <![endif]-->
<!--[if IE 7]>
<html class="no-js lt-ie9 lt-ie8" lang="en"> <![endif]-->
<!--[if IE 8]>
<html class="no-js lt-ie9" lang="en"> <![endif]-->
<!--[if gt IE 8]><!-->
<html class="no-js" lang="en"> <!--<![endif]-->
<head>
    <meta charset="UTF-8">
    <meta http-equiv="X-UA-Compatible" content="IE=edge,chrome=1">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <meta name="description" content="Muximux - Application Management Console">
    <link rel="shortcut icon" href="favicon.ico" type="image/ico"/>
    <link rel="stylesheet" type="text/css" href="css/cssreset.min.css"> <!-- Yahoo YUI HTML5 CSS reset -->
    <link rel="stylesheet" href="https://maxcdn.bootstrapcdn.com/bootstrap/3.3.6/css/bootstrap.min.css"
          integrity="sha384-1q8mTJOASx8j1Au+a5WDVnPi2lkFfwwEAa8hDDdjZlpLegxhjVME1fgjWPGmkzs7" crossorigin="anonymous">
    <link rel="stylesheet" href="css/bootstrap-iconpicker.min.css"/>
    <link rel="stylesheet" href="https://maxcdn.bootstrapcdn.com/font-awesome/4.5.0/css/font-awesome.min.css"/>
    <link rel="stylesheet" href="//fonts.googleapis.com/css?family=PT+Sans:400" type="text/css"> <!-- Font -->
    <link rel="stylesheet" href="css/style.css"> <!-- Resource style -->
    <link rel="stylesheet" href="css/jquery-ui.min.css">
    <script src="js/modernizr-2.8.3-respond-1.4.2.min.js"></script>
    <title><?php echo getTitle(); ?></title>
</head>

<body>
<!--[if lt IE 8]>
<p class="browserupgrade">You are using an <strong>outdated</strong> browser. Please <a href="http://browsehappy.com/">upgrade
    your browser</a> to improve your experience.</p>
<![endif]-->

<div class="cd-tabs">
    <?php echo menuItems(); ?>

    <ul class="cd-tabs-content">
        <?php echo frameContent(); ?>
    </ul>
</div>
<!-- Modal -->
<div id="settingsModal" class="modal fade" role="dialog">
    <div class="modal-dialog">

        <!-- Modal content-->
        <div class="modal-content">
            <div class="modal-header">
                <button type="button" class="close" data-dismiss="modal">&times;</button>
                <div class="modal-title"><h1>Settings</h1></div>
            </div>
            <div class="modal-body">
                <div class="text-center">
                    <div class="btn-group" role="group" aria-label="Buttons">
                        <a class="btn btn-primary" id="showInstructions"><span class="fa fa-book"></span> Show
                            Instructions</a>
                        <a class="btn btn-primary" id="showChangelog"><span class="fa fa-github"></span> Show
                            Updates</a>
                    </div>
                </div>

                <div id="instructionsContainer" class="alert alert-info">
                    <h3>Instructions</h3>
                    <p>The order that you put these blocks in determine in what order they will be listed in the
                        menu.<br>
                        Enable or disable each block and edit the URL to point to your desired location.<br/><br/></p>
                    <h3>Running Muximux from SSL-enabled / HTTPS server</h3>
                    <p>Please note that if Muximux is served via HTTPS, any services that are NOT served via HTTPS might
                        be blocked by your web-browser.<br><br>
                        Loading of unsecured content in the context of an SSL encrypted website where you see a green
                        lock would be misleading, therefore the browser blocks it.<br>
                        One work-around is to serve Muximux via an unsecured website, or to make sure all the
                        services/urls you link to use https://</p>

                    <p>Alternatively, if you use Chrome or Opera (or any Chromium-based browser), you can install
                        the plugin "Ignore X-Frame headers", which<br>
                        drops X-Frame-Options and Content-Security-Policy HTTP response headers, allowing ALL pages to
                        be
                        iframed (like we're doing in Muximux).</p>

                    <p>See:
                        <a href="https://chrome.google.com/webstore/detail/ignore-x-frame-headers/gleekbfjekiniecknbkamfmkohkpodhe"
                           target="_blank">https://chrome.google.com/webstore/detail/ignore-x-frame-headers/gleekbfjekiniecknbkamfmkohkpodhe</a>
                    </p>

                    <p>See <a href="https://github.com/mescon/Muximux/" target="_blank">https://github.com/mescon/Muximux/</a>
                        for more information.</p>

                </div>
                <div id="changelogContainer" class="alert alert-warning">
                    <h3>Updates</h3>
                    <div id="changelog"></div>
                </div>
                <div id="backupiniContainer" class="alert alert-warning">
                    <h3 style="text-align:center;">Backup.ini.php Contents</h3>
                    <div id="removeINIContainer"><button type="button" class="btn btn-primary" id="removeBackup">Remove Backup INI</button></div>
                    <div id="backupContents"><?php if (file_exists('backup.ini.php')) echo nl2br(file_get_contents('backup.ini.php')); ?></div>
                </div>

                <?php echo parse_ini(); ?>
            </div>
        </div>
        <div class="modal-footer">
            <button type="button" class="btn btn-default" data-dismiss="modal">Cancel</button>
            <button type='button' class="btn btn-primary" id='settingsSubmit' value='Submit Changes'>Save and Reload
            </button>
        </div>
    </div>
</div>
<div id="upgradeModal" class="modal fade" role="dialog">
    <div class="modal-dialog upgradeDialog">

        <!-- Modal content-->
        <div class="modal-content">
            <div class="modal-header">
                <button type="button" class="close" data-dismiss="modal">&times;</button>
                <div class="modal-title"><h1>Update Notification</h1></div>
            </div>
            <div class="modal-body upgradeBody">
                <div class="alert alert-info">
                    There has been an update. We removed <code>config.ini.php</code> and copied it into <code>backup.ini.php</code>
                    This is the last time we will have to do this kind of change.
                    This is due to the fact that we made major changes to the config.ini.php
                    and it is now called settings.ini.php. Do not copy your old config into
                    the new settings.ini.php. It needs to be written by the settings menu that
                    can be now be found in the dropdown in the top right. Thank you for your understanding.
                </div>
            </div>
        </div>
        <div class="modal-footer">
            <button type='button' class="btn btn-primary" data-dismiss="modal">Okay</button>
        </div>
    </div>
</div>

<script src="js/jquery-2.2.0.min.js"></script>
<script src="js/jquery-ui.min.js"></script>
<script src="js/jquery.form.min.js"></script>
<script src="//maxcdn.bootstrapcdn.com/bootstrap/3.3.6/js/bootstrap.min.js"></script>
<script type="text/javascript" src="js/iconset-fontawesome-4.2.0.min.js"></script>
<script type="text/javascript" src="js/bootstrap-iconpicker.min.js"></script>
<script type="text/javascript" src="js/main.js"></script>
<script type="text/javascript" src="js/functions.js"></script>
<?php if ($upgrade) echo "<script type=\"text/javascript\">$('#upgradeModal').modal();</script>"; ?>
<meta id='gitData'>
</body>
</html>
