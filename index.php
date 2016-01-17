<!doctype html>
<?php
function exception_error_handler($errno, $errstr, $errfile, $errline ) {
    throw new ErrorException($errstr, $errno, 0, $errfile, $errline);
}

set_error_handler("exception_error_handler");
try {
    include("muximux.php");
} catch (ErrorException $ex) {
    exit("Unable to load muximux.php.");
}
?>
<!--[if lt IE 7]>      <html class="no-js lt-ie9 lt-ie8 lt-ie7" lang="en"> <![endif]-->
<!--[if IE 7]>         <html class="no-js lt-ie9 lt-ie8" lang="en"> <![endif]-->
<!--[if IE 8]>         <html class="no-js lt-ie9" lang="en"> <![endif]-->
<!--[if gt IE 8]><!--> <html class="no-js" lang="en"> <!--<![endif]-->
<head>
    <meta charset="UTF-8">
    <meta http-equiv="X-UA-Compatible" content="IE=edge,chrome=1">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <meta name="description" content="Muximux - Application Management Console">
    <link rel="shortcut icon" href="favicon.ico" type="image/ico" />
    <link rel="stylesheet" href="https://maxcdn.bootstrapcdn.com/bootstrap/3.3.6/css/bootstrap.min.css" integrity="sha384-1q8mTJOASx8j1Au+a5WDVnPi2lkFfwwEAa8hDDdjZlpLegxhjVME1fgjWPGmkzs7" crossorigin="anonymous"> <!-- Bootstrap (includes Glyphicons) -->
    <link rel="stylesheet" href="https://maxcdn.bootstrapcdn.com/font-awesome/4.5.0/css/font-awesome.min.css" /> <!--FontAwesome-->
    <link rel="stylesheet" href="//fonts.googleapis.com/css?family=PT+Sans:400" type="text/css"> <!-- Font -->
    <link rel="stylesheet" href="css/reset.css"> <!-- CSS reset -->
    <link rel="stylesheet" href="css/style.css"> <!-- Resource style -->
    <script src="js/modernizr-2.8.3-respond-1.4.2.min.js"></script> <!-- Modernizr -->
    <title><?php echo getTitle($config); ?></title>
</head>

<body>
<!--[if lt IE 8]>
    <p class="browserupgrade">You are using an <strong>outdated</strong> browser. Please <a href="http://browsehappy.com/">upgrade your browser</a> to improve your experience.</p>
<![endif]-->

<div class="cd-tabs">

        <?php echo menuItems($config); ?>


    <ul class="cd-tabs-content">
        <?php echo frameContent($config); ?>
    </ul>
</div>

<script src="js/jquery-2.2.0.min.js"></script>
<script src="js/main.js"></script> <!-- Resource jQuery -->
</body>
</html>
