<!doctype html>
<?php
function exception_error_handler($errno, $errstr, $errfile, $errline)
{
    throw new ErrorException($errstr, $errno, 0, $errfile, $errline);
}

set_error_handler("exception_error_handler");
try {
    include("muximux.php");
} catch (ErrorException $ex) {
    exit("Unable to load muximux.php.");
}
try {
    include("settings.php");
} catch (ErrorException $ex) {
    exit("Unable to load settings.php.");
}
?>
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
          integrity="sha384-1q8mTJOASx8j1Au+a5WDVnPi2lkFfwwEAa8hDDdjZlpLegxhjVME1fgjWPGmkzs7" crossorigin="anonymous"> <!-- Bootstap (includes Glyphicons) -->
    <!-- Bootstrap-Iconpicker -->
    <link rel="stylesheet" href="css/bootstrap-iconpicker.min.css"/>
    <link rel="stylesheet" href="https://maxcdn.bootstrapcdn.com/font-awesome/4.5.0/css/font-awesome.min.css"/> <!--FontAwesome-->
    <link rel="stylesheet" href="//fonts.googleapis.com/css?family=PT+Sans:400" type="text/css"> <!-- Font -->
    <link rel="stylesheet" href="css/style.css"> <!-- Resource style -->
    <link rel="stylesheet" href="css/jquery-ui.min.css">
    <script src="js/modernizr-2.8.3-respond-1.4.2.min.js"></script>
    <!-- Modernizr -->
    <title><?php echo getTitle($config); ?></title>
</head>

<body>
<!--[if lt IE 8]>
<p class="browserupgrade">You are using an <strong>outdated</strong> browser. Please <a href="http://browsehappy.com/">upgrade
    your browser</a> to improve your experience.</p>
<![endif]-->

<div class="cd-tabs">
    <?php echo menuItems($config); ?>


    <ul class="cd-tabs-content">
        <?php echo frameContent($config); ?>
    </ul>
</div>
<!-- Modal -->
<div id="myModal" class="modal fade" role="dialog">
    <div class="modal-dialog">

        <!-- Modal content-->
        <div class="modal-content">
            <div class="modal-header">
                <button type="button" class="close" data-dismiss="modal">&times;</button>
                <div class="modal-title"><h1>Settings</h1></div>
            </div>
            <div class="modal-body">
                    <div id="instructions" class="alert alert-info">
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
                    drops X-Frame-Options and Content-Security-Policy HTTP response headers, allowing ALL pages to be
                    iframed (like we're doing in Muximux).</p>

                    <p>See:
                        <a href="https://chrome.google.com/webstore/detail/ignore-x-frame-headers/gleekbfjekiniecknbkamfmkohkpodhe" target="_blank">https://chrome.google.com/webstore/detail/ignore-x-frame-headers/gleekbfjekiniecknbkamfmkohkpodhe</a></p>

                    <p>See <a href="https://github.com/mescon/Muximux/" target="_blank">https://github.com/mescon/Muximux/</a> for more information.</p>

                </div>
                <div class="center" style="width: 140px;">
                    <button type="button" class="btn btn-primary btn-block" id="showInstructions">Show Instructions</button>
                </div>


                <?php echo parse_ini(); ?>
            </div>
            <div class="modal-footer">
                <button type="button" class="btn btn-default" data-dismiss="modal">Cancel</button>
                <button type='button' class="btn btn-primary" id='settingsSubmit' value='Submit Changes'>Save and Reload</button>

            </div>
        </div>

    </div>
</div>
<script src="js/jquery-2.2.0.min.js"></script>
<script src="js/jquery-ui.min.js"></script>
<script src="js/jquery.form.min.js"></script>
<script src="//maxcdn.bootstrapcdn.com/bootstrap/3.3.6/js/bootstrap.min.js"></script>
<script type="text/javascript" src="js/iconset-fontawesome-4.2.0.min.js"></script>
<script type="text/javascript" src="js/bootstrap-iconpicker.min.js"></script>
<script src="js/main.js"></script>
<!-- Resource jQuery -->
</body>
</html>
