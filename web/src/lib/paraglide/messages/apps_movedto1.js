/* eslint-disable */
import { getLocale, experimentalStaticLocale } from '../runtime.js';

/** @typedef {import('../runtime.js').LocalizedString} LocalizedString */

/** @typedef {{ name: NonNullable<unknown>, position: NonNullable<unknown>, total: NonNullable<unknown> }} Apps_Movedto1Inputs */

const en_apps_movedto1 = /** @type {(inputs: Apps_Movedto1Inputs) => LocalizedString} */ (i) => {
	return /** @type {LocalizedString} */ (`Moved ${i?.name} to position ${i?.position} of ${i?.total}`)
};

const ko_apps_movedto1 = /** @type {(inputs: Apps_Movedto1Inputs) => LocalizedString} */ (i) => {
	return /** @type {LocalizedString} */ (`${i?.name}이(가) ${i?.total}개 중 ${i?.position}번 위치로 이동되었습니다`)
};

/** @type {(inputs: Apps_Movedto1Inputs) => LocalizedString} */
const sv_apps_movedto1 = en_apps_movedto1;

/** @type {(inputs: Apps_Movedto1Inputs) => LocalizedString} */
const uk_apps_movedto1 = en_apps_movedto1;

/** @type {(inputs: Apps_Movedto1Inputs) => LocalizedString} */
const zh_apps_movedto1 = en_apps_movedto1;

/** @type {(inputs: Apps_Movedto1Inputs) => LocalizedString} */
const es_apps_movedto1 = en_apps_movedto1;

/** @type {(inputs: Apps_Movedto1Inputs) => LocalizedString} */
const hi_apps_movedto1 = en_apps_movedto1;

/** @type {(inputs: Apps_Movedto1Inputs) => LocalizedString} */
const pt_apps_movedto1 = en_apps_movedto1;

/** @type {(inputs: Apps_Movedto1Inputs) => LocalizedString} */
const bn_apps_movedto1 = en_apps_movedto1;

/** @type {(inputs: Apps_Movedto1Inputs) => LocalizedString} */
const ru_apps_movedto1 = en_apps_movedto1;

/** @type {(inputs: Apps_Movedto1Inputs) => LocalizedString} */
const ja_apps_movedto1 = en_apps_movedto1;

/** @type {(inputs: Apps_Movedto1Inputs) => LocalizedString} */
const vi_apps_movedto1 = en_apps_movedto1;

/** @type {(inputs: Apps_Movedto1Inputs) => LocalizedString} */
const yue_apps_movedto1 = en_apps_movedto1;

/** @type {(inputs: Apps_Movedto1Inputs) => LocalizedString} */
const tr_apps_movedto1 = en_apps_movedto1;

/** @type {(inputs: Apps_Movedto1Inputs) => LocalizedString} */
const ar_apps_movedto1 = en_apps_movedto1;

/** @type {(inputs: Apps_Movedto1Inputs) => LocalizedString} */
const wuu_apps_movedto1 = en_apps_movedto1;

/** @type {(inputs: Apps_Movedto1Inputs) => LocalizedString} */
const mr_apps_movedto1 = en_apps_movedto1;

/** @type {(inputs: Apps_Movedto1Inputs) => LocalizedString} */
const nb_apps_movedto1 = en_apps_movedto1;

/** @type {(inputs: Apps_Movedto1Inputs) => LocalizedString} */
const fi_apps_movedto1 = en_apps_movedto1;

/** @type {(inputs: Apps_Movedto1Inputs) => LocalizedString} */
const da_apps_movedto1 = en_apps_movedto1;

/** @type {(inputs: Apps_Movedto1Inputs) => LocalizedString} */
const et_apps_movedto1 = en_apps_movedto1;

/** @type {(inputs: Apps_Movedto1Inputs) => LocalizedString} */
const lv_apps_movedto1 = en_apps_movedto1;

/** @type {(inputs: Apps_Movedto1Inputs) => LocalizedString} */
const lt_apps_movedto1 = en_apps_movedto1;

/** @type {(inputs: Apps_Movedto1Inputs) => LocalizedString} */
const pl_apps_movedto1 = en_apps_movedto1;

/** @type {(inputs: Apps_Movedto1Inputs) => LocalizedString} */
const de_apps_movedto1 = en_apps_movedto1;

/** @type {(inputs: Apps_Movedto1Inputs) => LocalizedString} */
const nl_apps_movedto1 = en_apps_movedto1;

/** @type {(inputs: Apps_Movedto1Inputs) => LocalizedString} */
const fr_apps_movedto1 = en_apps_movedto1;

/** @type {(inputs: Apps_Movedto1Inputs) => LocalizedString} */
const it_apps_movedto1 = en_apps_movedto1;

/** @type {(inputs: Apps_Movedto1Inputs) => LocalizedString} */
const hu_apps_movedto1 = en_apps_movedto1;

/** @type {(inputs: Apps_Movedto1Inputs) => LocalizedString} */
const cs_apps_movedto1 = en_apps_movedto1;

/** @type {(inputs: Apps_Movedto1Inputs) => LocalizedString} */
const ro_apps_movedto1 = en_apps_movedto1;

/** @type {(inputs: Apps_Movedto1Inputs) => LocalizedString} */
const el_apps_movedto1 = en_apps_movedto1;

/** @type {(inputs: Apps_Movedto1Inputs) => LocalizedString} */
const bg_apps_movedto1 = en_apps_movedto1;

/** @type {(inputs: Apps_Movedto1Inputs) => LocalizedString} */
const hr_apps_movedto1 = en_apps_movedto1;

/** @type {(inputs: Apps_Movedto1Inputs) => LocalizedString} */
const sr_apps_movedto1 = en_apps_movedto1;

/** @type {(inputs: Apps_Movedto1Inputs) => LocalizedString} */
const sk_apps_movedto1 = en_apps_movedto1;

/** @type {(inputs: Apps_Movedto1Inputs) => LocalizedString} */
const sl_apps_movedto1 = en_apps_movedto1;

/**
* | output |
* | --- |
* | "Moved {name} to position {position} of {total}" |
*
* @param {Apps_Movedto1Inputs} inputs
* @param {{ locale?: "en" | "sv" | "uk" | "zh" | "es" | "hi" | "pt" | "bn" | "ru" | "ja" | "vi" | "yue" | "tr" | "ar" | "wuu" | "mr" | "nb" | "fi" | "da" | "et" | "lv" | "lt" | "pl" | "de" | "nl" | "fr" | "it" | "hu" | "cs" | "ro" | "el" | "bg" | "hr" | "sr" | "sk" | "sl" | "ko" }} options
* @returns {LocalizedString}
*/
const apps_movedto1 = /** @type {((inputs: Apps_Movedto1Inputs, options?: { locale?: "en" | "sv" | "uk" | "zh" | "es" | "hi" | "pt" | "bn" | "ru" | "ja" | "vi" | "yue" | "tr" | "ar" | "wuu" | "mr" | "nb" | "fi" | "da" | "et" | "lv" | "lt" | "pl" | "de" | "nl" | "fr" | "it" | "hu" | "cs" | "ro" | "el" | "bg" | "hr" | "sr" | "sk" | "sl" | "ko" }) => LocalizedString) & import('../runtime.js').MessageMetadata<Apps_Movedto1Inputs, { locale?: "en" | "sv" | "uk" | "zh" | "es" | "hi" | "pt" | "bn" | "ru" | "ja" | "vi" | "yue" | "tr" | "ar" | "wuu" | "mr" | "nb" | "fi" | "da" | "et" | "lv" | "lt" | "pl" | "de" | "nl" | "fr" | "it" | "hu" | "cs" | "ro" | "el" | "bg" | "hr" | "sr" | "sk" | "sl" | "ko" }, {}>} */ ((inputs, options = {}) => {
	const locale = experimentalStaticLocale ?? options.locale ?? getLocale()
	if (locale === "en") return en_apps_movedto1(inputs)
	if (locale === "sv") return sv_apps_movedto1(inputs)
	if (locale === "uk") return uk_apps_movedto1(inputs)
	if (locale === "zh") return zh_apps_movedto1(inputs)
	if (locale === "es") return es_apps_movedto1(inputs)
	if (locale === "hi") return hi_apps_movedto1(inputs)
	if (locale === "pt") return pt_apps_movedto1(inputs)
	if (locale === "bn") return bn_apps_movedto1(inputs)
	if (locale === "ru") return ru_apps_movedto1(inputs)
	if (locale === "ja") return ja_apps_movedto1(inputs)
	if (locale === "vi") return vi_apps_movedto1(inputs)
	if (locale === "yue") return yue_apps_movedto1(inputs)
	if (locale === "tr") return tr_apps_movedto1(inputs)
	if (locale === "ar") return ar_apps_movedto1(inputs)
	if (locale === "wuu") return wuu_apps_movedto1(inputs)
	if (locale === "mr") return mr_apps_movedto1(inputs)
	if (locale === "nb") return nb_apps_movedto1(inputs)
	if (locale === "fi") return fi_apps_movedto1(inputs)
	if (locale === "da") return da_apps_movedto1(inputs)
	if (locale === "et") return et_apps_movedto1(inputs)
	if (locale === "lv") return lv_apps_movedto1(inputs)
	if (locale === "lt") return lt_apps_movedto1(inputs)
	if (locale === "pl") return pl_apps_movedto1(inputs)
	if (locale === "de") return de_apps_movedto1(inputs)
	if (locale === "nl") return nl_apps_movedto1(inputs)
	if (locale === "fr") return fr_apps_movedto1(inputs)
	if (locale === "it") return it_apps_movedto1(inputs)
	if (locale === "hu") return hu_apps_movedto1(inputs)
	if (locale === "cs") return cs_apps_movedto1(inputs)
	if (locale === "ro") return ro_apps_movedto1(inputs)
	if (locale === "el") return el_apps_movedto1(inputs)
	if (locale === "bg") return bg_apps_movedto1(inputs)
	if (locale === "hr") return hr_apps_movedto1(inputs)
	if (locale === "sr") return sr_apps_movedto1(inputs)
	if (locale === "sk") return sk_apps_movedto1(inputs)
	if (locale === "sl") return sl_apps_movedto1(inputs)
	return ko_apps_movedto1(inputs)
});
export { apps_movedto1 as "apps_movedTo" }