<?php defined('DS') OR die('No direct access allowed.');
defined("CONFIG") ? null : define('CONFIG', 'settings.ini.php');
defined("CONFIGEXAMPLE") ? null : define('CONFIGEXAMPLE', 'settings.ini.php-example');
defined("SECRET") ? null : define('SECRET', 'secret.txt');
require dirname(__FILE__) . '/vendor/autoload.php';
$config = new Config_Lite(CONFIG);
$hash = $config->get('general', 'password', '0');
$title = $config->get('general', 'title', '0');
$username = $config->get('general', 'userNameInput', '0');
if(isset($_GET['logout'])) {
    $_SESSION['username'] = '';
	if (!is_session_started()) session_start();
	session_destroy();
    header('Location:  ' . $_SERVER['PHP_SELF']);
	console_log('Destroying session for logout');
}

if(isset($_POST['username'])) {

    if ($_POST['username'] == $username && password_verify($_POST['password'],$hash)) {

            $_SESSION['username'] = $_POST['username'];
            header("Location: " . "http://" . $_SERVER['HTTP_HOST']);
			session_start();
    } else {
			console_log('Error validating login of ' . $username . ' with password of ' . $_POST['password']);
		
    }
	console_log('Got username');
} else {
	console_log('Post received, but username is not set');
}

echo '
    <meta http-equiv="X-UA-Compatible" content="IE=edge,chrome=1">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <link rel="stylesheet" href="css/login.css">
    <link rel="stylesheet" href="css/theme/'.getTheme().'.css">
    <link rel="stylesheet" href="css/font-awesome.min.css"/>
    <title>Login to '.getTitle().'</title>
    <script src="js/login.js"></script>
</head>
<body>
<div class="wrapper">
    <div class="logo">
		'.
	file_get_contents("images/muximux-white.svg")
		
	.'</div>
    <div class="login-block" id="slide">
		<form method="post" id=login action="index.php">
        <h1 class="login0">Login</h1>
            <input type="text" class="login1" value="" placeholder="Username" id="username" name="username" value="" />
            <input type="password" class="login2" value="" placeholder="Password" id="password"  name="password" value="" />
            <input type="submit" class="login3" name="submit" value="Login" class="button"/></p>
        </form>
    </div>
	</div>
</body>
';
exit;
?>