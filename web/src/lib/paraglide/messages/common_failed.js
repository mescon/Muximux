/* eslint-disable */
import { getLocale, experimentalStaticLocale } from '../runtime.js';

/** @typedef {import('../runtime.js').LocalizedString} LocalizedString */

/** @typedef {{}} Common_FailedInputs */

const en_common_failed = /** @type {(inputs: Common_FailedInputs) => LocalizedString} */ () => {
	return /** @type {LocalizedString} */ (`Failed`)
};

const sv_common_failed = /** @type {(inputs: Common_FailedInputs) => LocalizedString} */ () => {
	return /** @type {LocalizedString} */ (`Misslyckades`)
};

const uk_common_failed = /** @type {(inputs: Common_FailedInputs) => LocalizedString} */ () => {
	return /** @type {LocalizedString} */ (`Не вдалося`)
};

const zh_common_failed = /** @type {(inputs: Common_FailedInputs) => LocalizedString} */ () => {
	return /** @type {LocalizedString} */ (`失败`)
};

const es_common_failed = /** @type {(inputs: Common_FailedInputs) => LocalizedString} */ () => {
	return /** @type {LocalizedString} */ (`Error`)
};

const hi_common_failed = /** @type {(inputs: Common_FailedInputs) => LocalizedString} */ () => {
	return /** @type {LocalizedString} */ (`विफल`)
};

const pt_common_failed = /** @type {(inputs: Common_FailedInputs) => LocalizedString} */ () => {
	return /** @type {LocalizedString} */ (`Falhou`)
};

const bn_common_failed = /** @type {(inputs: Common_FailedInputs) => LocalizedString} */ () => {
	return /** @type {LocalizedString} */ (`ব্যর্থ`)
};

const ru_common_failed = /** @type {(inputs: Common_FailedInputs) => LocalizedString} */ () => {
	return /** @type {LocalizedString} */ (`Ошибка`)
};

const ja_common_failed = /** @type {(inputs: Common_FailedInputs) => LocalizedString} */ () => {
	return /** @type {LocalizedString} */ (`失敗`)
};

const vi_common_failed = /** @type {(inputs: Common_FailedInputs) => LocalizedString} */ () => {
	return /** @type {LocalizedString} */ (`Thất bại`)
};

const yue_common_failed = /** @type {(inputs: Common_FailedInputs) => LocalizedString} */ () => {
	return /** @type {LocalizedString} */ (`失敗`)
};

const tr_common_failed = /** @type {(inputs: Common_FailedInputs) => LocalizedString} */ () => {
	return /** @type {LocalizedString} */ (`Başarısız`)
};

const ar_common_failed = /** @type {(inputs: Common_FailedInputs) => LocalizedString} */ () => {
	return /** @type {LocalizedString} */ (`فشل`)
};

const wuu_common_failed = /** @type {(inputs: Common_FailedInputs) => LocalizedString} */ () => {
	return /** @type {LocalizedString} */ (`失败`)
};

const mr_common_failed = /** @type {(inputs: Common_FailedInputs) => LocalizedString} */ () => {
	return /** @type {LocalizedString} */ (`अयशस्वी`)
};

const nb_common_failed = /** @type {(inputs: Common_FailedInputs) => LocalizedString} */ () => {
	return /** @type {LocalizedString} */ (`Mislyktes`)
};

const fi_common_failed = /** @type {(inputs: Common_FailedInputs) => LocalizedString} */ () => {
	return /** @type {LocalizedString} */ (`Epäonnistui`)
};

const da_common_failed = /** @type {(inputs: Common_FailedInputs) => LocalizedString} */ () => {
	return /** @type {LocalizedString} */ (`Mislykkedes`)
};

const et_common_failed = /** @type {(inputs: Common_FailedInputs) => LocalizedString} */ () => {
	return /** @type {LocalizedString} */ (`Ebaõnnestus`)
};

const lv_common_failed = /** @type {(inputs: Common_FailedInputs) => LocalizedString} */ () => {
	return /** @type {LocalizedString} */ (`Neizdevās`)
};

const lt_common_failed = /** @type {(inputs: Common_FailedInputs) => LocalizedString} */ () => {
	return /** @type {LocalizedString} */ (`Nepavyko`)
};

const pl_common_failed = /** @type {(inputs: Common_FailedInputs) => LocalizedString} */ () => {
	return /** @type {LocalizedString} */ (`Nie powiodło się`)
};

const de_common_failed = /** @type {(inputs: Common_FailedInputs) => LocalizedString} */ () => {
	return /** @type {LocalizedString} */ (`Fehlgeschlagen`)
};

const nl_common_failed = /** @type {(inputs: Common_FailedInputs) => LocalizedString} */ () => {
	return /** @type {LocalizedString} */ (`Mislukt`)
};

const fr_common_failed = /** @type {(inputs: Common_FailedInputs) => LocalizedString} */ () => {
	return /** @type {LocalizedString} */ (`Échoué`)
};

const it_common_failed = /** @type {(inputs: Common_FailedInputs) => LocalizedString} */ () => {
	return /** @type {LocalizedString} */ (`Non riuscito`)
};

const hu_common_failed = /** @type {(inputs: Common_FailedInputs) => LocalizedString} */ () => {
	return /** @type {LocalizedString} */ (`Sikertelen`)
};

const cs_common_failed = /** @type {(inputs: Common_FailedInputs) => LocalizedString} */ () => {
	return /** @type {LocalizedString} */ (`Selhalo`)
};

const ro_common_failed = /** @type {(inputs: Common_FailedInputs) => LocalizedString} */ () => {
	return /** @type {LocalizedString} */ (`Eșuat`)
};

const el_common_failed = /** @type {(inputs: Common_FailedInputs) => LocalizedString} */ () => {
	return /** @type {LocalizedString} */ (`Απέτυχε`)
};

const bg_common_failed = /** @type {(inputs: Common_FailedInputs) => LocalizedString} */ () => {
	return /** @type {LocalizedString} */ (`Неуспешно`)
};

const hr_common_failed = /** @type {(inputs: Common_FailedInputs) => LocalizedString} */ () => {
	return /** @type {LocalizedString} */ (`Neuspjelo`)
};

const sr_common_failed = /** @type {(inputs: Common_FailedInputs) => LocalizedString} */ () => {
	return /** @type {LocalizedString} */ (`Neuspelo`)
};

const sk_common_failed = /** @type {(inputs: Common_FailedInputs) => LocalizedString} */ () => {
	return /** @type {LocalizedString} */ (`Zlyhalo`)
};

const sl_common_failed = /** @type {(inputs: Common_FailedInputs) => LocalizedString} */ () => {
	return /** @type {LocalizedString} */ (`Neuspelo`)
};

/**
* | output |
* | --- |
* | "Failed" |
*
* @param {Common_FailedInputs} inputs
* @param {{ locale?: "en" | "sv" | "uk" | "zh" | "es" | "hi" | "pt" | "bn" | "ru" | "ja" | "vi" | "yue" | "tr" | "ar" | "wuu" | "mr" | "nb" | "fi" | "da" | "et" | "lv" | "lt" | "pl" | "de" | "nl" | "fr" | "it" | "hu" | "cs" | "ro" | "el" | "bg" | "hr" | "sr" | "sk" | "sl" }} options
* @returns {LocalizedString}
*/
export const common_failed = /** @type {((inputs?: Common_FailedInputs, options?: { locale?: "en" | "sv" | "uk" | "zh" | "es" | "hi" | "pt" | "bn" | "ru" | "ja" | "vi" | "yue" | "tr" | "ar" | "wuu" | "mr" | "nb" | "fi" | "da" | "et" | "lv" | "lt" | "pl" | "de" | "nl" | "fr" | "it" | "hu" | "cs" | "ro" | "el" | "bg" | "hr" | "sr" | "sk" | "sl" }) => LocalizedString) & import('../runtime.js').MessageMetadata<Common_FailedInputs, { locale?: "en" | "sv" | "uk" | "zh" | "es" | "hi" | "pt" | "bn" | "ru" | "ja" | "vi" | "yue" | "tr" | "ar" | "wuu" | "mr" | "nb" | "fi" | "da" | "et" | "lv" | "lt" | "pl" | "de" | "nl" | "fr" | "it" | "hu" | "cs" | "ro" | "el" | "bg" | "hr" | "sr" | "sk" | "sl" }, {}>} */ ((inputs = {}, options = {}) => {
	const locale = experimentalStaticLocale ?? options.locale ?? getLocale()
	if (locale === "en") return en_common_failed(inputs)
	if (locale === "sv") return sv_common_failed(inputs)
	if (locale === "uk") return uk_common_failed(inputs)
	if (locale === "zh") return zh_common_failed(inputs)
	if (locale === "es") return es_common_failed(inputs)
	if (locale === "hi") return hi_common_failed(inputs)
	if (locale === "pt") return pt_common_failed(inputs)
	if (locale === "bn") return bn_common_failed(inputs)
	if (locale === "ru") return ru_common_failed(inputs)
	if (locale === "ja") return ja_common_failed(inputs)
	if (locale === "vi") return vi_common_failed(inputs)
	if (locale === "yue") return yue_common_failed(inputs)
	if (locale === "tr") return tr_common_failed(inputs)
	if (locale === "ar") return ar_common_failed(inputs)
	if (locale === "wuu") return wuu_common_failed(inputs)
	if (locale === "mr") return mr_common_failed(inputs)
	if (locale === "nb") return nb_common_failed(inputs)
	if (locale === "fi") return fi_common_failed(inputs)
	if (locale === "da") return da_common_failed(inputs)
	if (locale === "et") return et_common_failed(inputs)
	if (locale === "lv") return lv_common_failed(inputs)
	if (locale === "lt") return lt_common_failed(inputs)
	if (locale === "pl") return pl_common_failed(inputs)
	if (locale === "de") return de_common_failed(inputs)
	if (locale === "nl") return nl_common_failed(inputs)
	if (locale === "fr") return fr_common_failed(inputs)
	if (locale === "it") return it_common_failed(inputs)
	if (locale === "hu") return hu_common_failed(inputs)
	if (locale === "cs") return cs_common_failed(inputs)
	if (locale === "ro") return ro_common_failed(inputs)
	if (locale === "el") return el_common_failed(inputs)
	if (locale === "bg") return bg_common_failed(inputs)
	if (locale === "hr") return hr_common_failed(inputs)
	if (locale === "sr") return sr_common_failed(inputs)
	if (locale === "sk") return sk_common_failed(inputs)
	return sl_common_failed(inputs)
});