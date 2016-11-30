<?php defined('DS') OR die('No direct access allowed.');
define('CONFIG', 'settings.ini.php');
define('CONFIGEXAMPLE', 'settings.ini.php-example');
define('SECRET', 'secret.txt');
require dirname(__FILE__) . '/vendor/autoload.php';
$config = new Config_Lite(CONFIG);
$hash = $config->get('general', 'password', '0');
$title = $config->get('general', 'title', '0');
$username = $config->get('general', 'userNameInput', '0');
if(isset($_GET['logout'])) {
    $_SESSION['username'] = '';
    header('Location:  ' . $_SERVER['PHP_SELF']);
}

if(isset($_POST['username'])) {
		
    if ($_POST['username'] == $username && password_verify($_POST['password'],$hash)) {
		
			$_SESSION['username'] = $_POST['username'];
			echo $_SERVER['HTTP_HOST'];
			header("Location: " . "http://" . $_SERVER['HTTP_HOST']);
		
    } else {
        //invalid login
		echo "<p>error logging in</p>";
    }
}

echo '
<link rel="stylesheet" href="css/font-awesome.min.css"/>
<link rel="stylesheet" href="css/login.css"> 
<script src="js/jquery-2.2.4.min.js"></script>
<script>
$(document).ready(function() {
    $(\'.login-block\').animate({top: "30%"}, 500, function() {
		$(\'.logo\').delay(1200).show(0);
    // Animation complete.
  });
  
});
</script>
<div class="logo">'. $title .'</div>
	<div class="login-block" id="slide">
		<form method="post" action="index.php">
		<h1>Login</h1>
			<input type="text" value="" placeholder="Username" id="username" name="username" value="" />
			<input type="password" value="" placeholder="Password" id="password"  name="password" value="" />
			<input type="submit" name="submit" value="Login" class="button"/></p>
		</form>
	</div>
';
exit; 
?>