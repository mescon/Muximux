<?php defined('DS') OR die('No direct access allowed.');
define('CONFIG', 'settings.ini.php');
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
            header("Location: " . "http://" . $_SERVER['HTTP_HOST']);

    } else {
        //invalid login
        echo "<p>error logging in</p>";
    }
}

echo '
    <meta http-equiv="X-UA-Compatible" content="IE=edge,chrome=1">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <link rel="stylesheet" href="css/login.css">
    <link rel="stylesheet" href="css/theme/'.getTheme().'.css">
    <link rel="stylesheet" href="css/font-awesome.min.css"/>
    <title>Login to '.getTitle().'</title>
    <script>
    $(document).ready(function() {
        $(\'.login-block\').animate({top: "30%"}, 100, \'easeInOutElastic\', function() {
            $(\'.logo\').delay(1200).slideDown(400,\'easeInOutElastic\');
            $(\'.login0\').delay(1000).slideDown(400,\'easeInOutElastic\');
            $(\'.login1\').delay(1100).slideDown(400,\'easeInOutElastic\');
            $(\'.login2\').delay(1200).slideDown(400,\'easeInOutElastic\');
            $(\'.login3\').delay(1300).slideDown(400,\'easeInOutElastic\');
        // Animation complete.
        });

    });
    </script>
</head>
<body>
    <div class="logo"><img src="images/muximux.png" alt="Muximux" width="235" height="128" /></div>
    <div class="login-block" id="slide">
        <form method="post" action="index.php">
        <h1 class="login0">Login</h1>
            <input type="text" class="login1" value="" placeholder="Username" id="username" name="username" value="" />
            <input type="password" class="login2" value="" placeholder="Password" id="password"  name="password" value="" />
            <input type="submit" class="login3" name="submit" value="Login" class="button"/></p>
        </form>
    </div>
</body>
';
exit;
?>