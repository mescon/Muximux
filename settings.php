<html>
<script src="js/jquery-2.2.0.min.js"></script>
<script src="js/main.js"></script> <!-- Resource jQuery -->
</html>
<?php
/**
 * Created by PhpStorm.
 * User: synik
 * Date: 1/18/2016
 * Time: 9:19 PM
 */
require __DIR__ . '/vendor/autoload.php';
$config = new Config_Lite('config.ini.php');
if(sizeof($_POST)==0) {
    parse_ini($config);
}
else{
    save_ini($_POST,$config);
}

function save_ini($postData,$config){
    foreach($postData as $parameter=>$value) {
        if ($parameter != "ignore") {
            $splitParameter = explode('-', $parameter);
            if ($value == "on")
                $value = "true";
            $config->set($splitParameter[0], $splitParameter[1], $value);
        }
    }
    $config->save();
    $updatedConfig = new Config_Lite('config.ini.php');
    parse_ini($updatedConfig);
}

function parse_ini($config)
{

    $pageOutput = "<form method='post' action='settings.php'>";

    $pageOutput .= "General:<br><div>Title: <input type='text' value='" . $config->get('general', 'title') . "'></div>";
    $pageOutput .= "Enable Dropdown: <input type='checkbox' ";
    if ($config->get('general', 'enabledropdown') == true)
        $pageOutput .= "checked></div><br><br>";
    else
        $pageOutput .= "></div><br>";

        foreach ($config as $section => $name) {
            if (is_array($name) && $section != "settings" && $section != "general") {
                $pageOutput .= "<br><div>Application: <input class='applicationName' type='text' value='".$section."'></div>";
                foreach ($name as $key => $val) {
                    if($key == "name" || $key == "url" || $key == "icon")
                        $pageOutput .= "<div>$key:<input class='".$section."-value' name='".$section."-".$key."' type='text' value='".$val."'></div></div>";
                    else{
                        $pageOutput .= "<div>$key<input class='checkbox ".$section."-value' name='".$section."-".$key."' type='checkbox' ";
                        if ($val == "true")
                            $pageOutput .= " checked></div></div>";
                        else
                            $pageOutput .= "></div></div>";
                    }
                }
            }
        }
    $pageOutput .= "<input type='submit' id='settingsSubmit' value='submit'></form>";
    echo $pageOutput;
}