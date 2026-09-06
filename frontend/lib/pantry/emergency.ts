/**
 * The German federal preparedness checklist, and the groups its food half is
 * counted in.
 *
 * Source: "Vorsorgen für Krisen und Katastrophen", Bundesamt für
 * Bevölkerungsschutz und Katastrophenhilfe (BBK), checklist pages.
 * https://www.bbk.bund.de/DE/Warnung-Vorsorge/Vorsorge/Ratgeber-Checkliste/ratgeber-checkliste_node.html
 *
 * The German wording is quoted rather than rewritten: it is an official list,
 * and paraphrasing it while still calling it official would be dishonest. The
 * English alongside is a plain translation for readability, not an official
 * version - the BBK publishes this in German.
 *
 * Item ids are stable and stored per group. Never renumber them: an id that
 * changes meaning turns somebody's ticked box into a lie about what they own.
 */

export type EmergencyCategory = "drinks" | "vegetables" | "grains" | "fruit" | "dairy" | "protein" | "fats";

/** The order the balance is reported in - biggest target first. */
export const categoryOrder: EmergencyCategory[] = [
  "drinks",
  "vegetables",
  "grains",
  "fruit",
  "dairy",
  "protein",
  "fats",
];

export interface ChecklistItem {
  id: string;
  de: string;
  en: string;
}

export interface ChecklistSection {
  id: string;
  de: string;
  en: string;
  items: ChecklistItem[];
}

export const checklist: ChecklistSection[] = [
  {
    id: "cooking",
    de: "Essen und Trinken",
    en: "Food and drink",
    items: [
      {
        id: "stove",
        de: "Alternative Kochgelegenheit (wie Camping-Kocher oder Gasgrill)",
        en: "An alternative way to cook (camping stove, gas grill)",
      },
      { id: "stove-fuel", de: "Brennstoffe für Kochgelegenheit (wie Gas)", en: "Fuel for it (gas)" },
      { id: "pet-food", de: "Futtervorrat für Haustiere", en: "Food for pets" },
    ],
  },
  {
    id: "information",
    de: "Information und Kommunikation",
    en: "Information and communication",
    items: [
      {
        id: "radio",
        de: "Solar- oder batteriebetriebenes Radio inkl. Batterien oder Kurbelradio",
        en: "Solar, battery or wind-up radio, with batteries",
      },
      { id: "warn-app", de: "Warn-App installiert (wie NINA)", en: "Warning app installed (such as NINA)" },
      { id: "powerbank", de: "Aufgeladene Powerbank", en: "A charged power bank" },
      {
        id: "phone-numbers",
        de: "Liste wichtiger Telefonnummern auf Papier",
        en: "Important phone numbers on paper",
      },
    ],
  },
  {
    id: "light",
    de: "Licht und Wärme",
    en: "Light and warmth",
    items: [
      { id: "torch", de: "Taschenlampe und Ersatzbatterien", en: "Torch and spare batteries" },
      { id: "candles", de: "Kerzen und Feuerzeug", en: "Candles and a lighter" },
      { id: "warm-clothes", de: "Warme Kleidung und Decken", en: "Warm clothes and blankets" },
      { id: "sleeping-bags", de: "Schlafsäcke", en: "Sleeping bags" },
      {
        id: "heater",
        de: "Netzunabhängige Heizgelegenheit (wie Gasheizer, Petroleumofen, Ethanolkamin)",
        en: "Heating that does not need mains power (gas heater, paraffin stove, ethanol fire)",
      },
      { id: "heater-fuel", de: "Brennstoffe für Heizgelegenheit", en: "Fuel for it" },
    ],
  },
  {
    id: "medicine",
    de: "Hausapotheke",
    en: "Medicine cabinet",
    items: [
      { id: "own-meds", de: "Persönliche Medikamente", en: "Your own medication" },
      { id: "painkillers", de: "Schmerzmittel", en: "Painkillers" },
      { id: "fever", de: "Fiebersenkende Mittel", en: "Something to bring a fever down" },
      { id: "thermometer", de: "Fieberthermometer", en: "Thermometer" },
      {
        id: "stomach",
        de: "Mittel gegen Durchfall, Erbrechen und Übelkeit",
        en: "Something for diarrhoea, vomiting and nausea",
      },
      { id: "cold", de: "Mittel gegen Erkältungsbeschwerden", en: "Something for a cold" },
      {
        id: "electrolytes",
        de: "Elektrolyte zum Ausgleich von Flüssigkeitsverlust",
        en: "Electrolytes to replace lost fluids",
      },
      { id: "wound-disinfectant", de: "Wunddesinfektionsmittel", en: "Wound disinfectant" },
      { id: "skin-disinfectant", de: "Hautdesinfektionsmittel", en: "Skin disinfectant" },
      { id: "plasters", de: "Pflaster und Verbandsmaterial", en: "Plasters and dressings" },
      { id: "burn-dressing", de: "Verbandtuch für Brandwunden", en: "Burn dressing" },
      { id: "gloves", de: "Einmalhandschuhe", en: "Disposable gloves" },
      { id: "cold-pack", de: "Kühlkompresse", en: "Cold pack" },
      {
        id: "sunburn",
        de: "Mittel gegen Sonnenbrand und Insektenstiche",
        en: "Something for sunburn and insect bites",
      },
      {
        id: "sports-gel",
        de: "Abschwellendes und kühlendes Gel für kleinere Sportverletzungen",
        en: "Cooling gel for minor sprains",
      },
      { id: "nasal-spray", de: "Abschwellende Nasentropfen oder Nasenspray", en: "Decongestant nasal spray" },
      { id: "tweezers", de: "Pinzette", en: "Tweezers" },
      { id: "scissors", de: "Schere", en: "Scissors" },
      { id: "ointment", de: "Brand-, Wund-, Heilsalbe", en: "Burn, wound and healing ointment" },
    ],
  },
  {
    id: "hygiene",
    de: "Hygiene",
    en: "Hygiene",
    items: [
      { id: "toothpaste", de: "Zahnpasta, Zahnbürsten", en: "Toothpaste and toothbrushes" },
      { id: "soap", de: "Seife", en: "Soap" },
      {
        id: "personal-hygiene",
        de: "Persönliche Hygieneartikel (wie Monatshygiene, Windeln)",
        en: "Personal hygiene supplies (sanitary products, nappies)",
      },
      {
        id: "bin-bags",
        de: "Müllbeutel, auch große und stabile zum Einhängen in die Toilette",
        en: "Bin bags, including large sturdy ones to line the toilet",
      },
      { id: "toilet-paper", de: "Toilettenpapier", en: "Toilet paper" },
      { id: "disinfectant", de: "Desinfektionsmittel", en: "Disinfectant" },
      { id: "detergent", de: "Waschmittel", en: "Washing detergent" },
      { id: "wet-wipes", de: "Feuchttücher", en: "Wet wipes" },
      { id: "kitchen-roll", de: "Haushaltspapier", en: "Kitchen roll" },
      { id: "household-gloves", de: "Haushaltshandschuhe", en: "Household gloves" },
      { id: "camping-toilet", de: "Campingtoilette", en: "Camping toilet" },
    ],
  },
  {
    id: "fire",
    de: "Brandschutz",
    en: "Fire safety",
    items: [
      { id: "smoke-alarm", de: "Rauchmelder", en: "Smoke alarm" },
      { id: "extinguisher", de: "Feuerlöscher/Feuerlöschspray", en: "Fire extinguisher or spray" },
      { id: "co-alarm", de: "Kohlenmonoxid-Melder", en: "Carbon monoxide alarm" },
    ],
  },
  {
    id: "gobag",
    de: "Notgepäck",
    en: "Go bag",
    items: [
      {
        id: "outdoor-clothes",
        de: "Warme Kleidung, Regenschutz und feste Schuhe",
        en: "Warm clothes, rainwear and sturdy shoes",
      },
      { id: "spare-clothes", de: "Wechselkleidung", en: "A change of clothes" },
      { id: "first-aid", de: "Erste-Hilfe-Material", en: "First aid supplies" },
      { id: "gobag-meds", de: "Persönliche Medikamente", en: "Your own medication" },
      { id: "gobag-powerbank", de: "Geladene Powerbank", en: "A charged power bank" },
      { id: "gobag-hygiene", de: "Hygieneartikel", en: "Toiletries" },
      {
        id: "gobag-food",
        de: "Haltbare Lebensmittel und wiederbefüllbare Trinkflasche",
        en: "Food that keeps, and a refillable bottle",
      },
      { id: "gobag-documents", de: "Mappe mit wichtigen Dokumenten", en: "A folder of important documents" },
      { id: "sleeping-bag", de: "Schlafsack oder Decke", en: "Sleeping bag or blanket" },
      { id: "crockery", de: "Essgeschirr", en: "Something to eat off" },
      { id: "knife", de: "Taschenmesser und Dosenöffner", en: "Pocket knife and tin opener" },
      { id: "gobag-torch", de: "Taschenlampe", en: "Torch" },
      {
        id: "gobag-radio",
        de: "Batterie- oder solarbetriebenes Radio bzw. Kurbelradio",
        en: "Battery, solar or wind-up radio",
      },
      { id: "matches", de: "Feuerzeug oder Streichhölzer", en: "Lighter or matches" },
      { id: "sun", de: "Sonnencreme und Kopfbedeckung", en: "Sun cream and a hat" },
      { id: "notepad", de: "Notizblock und Stift", en: "Notepad and pen" },
      { id: "work-gloves", de: "Arbeitshandschuhe", en: "Work gloves" },
      {
        id: "spare-aids",
        de: "Ersatz für Hilfsmittel wie Brille oder Hörsysteme",
        en: "Spares for glasses or hearing aids",
      },
      { id: "gobag-cash", de: "Bargeld", en: "Cash" },
    ],
  },
  {
    id: "documents",
    de: "Dokumente sicher aufbewahren",
    en: "Documents kept safe",
    items: [
      {
        id: "doc-identity",
        de: "Dokumente, die die Identität belegen (Geburtsurkunde, Personalausweis)",
        en: "Documents proving who you are (birth certificate, ID card)",
      },
      {
        id: "doc-property",
        de: "Dokumente, die Besitz belegen (Grundbucheinträge, Kaufverträge)",
        en: "Documents proving what you own (land register entries, contracts of sale)",
      },
      {
        id: "doc-financial",
        de: "Dokumente, die finanzielle Ansprüche belegen (Versicherungen)",
        en: "Documents proving financial claims (insurance)",
      },
      {
        id: "doc-qualifications",
        de: "Dokumente, die Qualifikationen belegen (Schul- und Arbeitszeugnisse)",
        en: "Documents proving qualifications (school and work references)",
      },
      {
        id: "doc-rights",
        de: "Dokumente, die Rechte belegen (Vollmachten, gerichtliche Entscheidungen)",
        en: "Documents proving rights (powers of attorney, court decisions)",
      },
      {
        id: "doc-personal",
        de: "Wichtige persönliche Unterlagen (Testament, medizinische Befunde, Impfausweis)",
        en: "Important personal papers (will, medical records, vaccination card)",
      },
      {
        id: "doc-copies",
        de: "Digitale Kopien in der Cloud, auf Festplatte oder USB-Stick",
        en: "Digital copies in the cloud, on a drive or a USB stick",
      },
      {
        id: "doc-container",
        de: "Aufbewahrung in einem feuerfesten oder wasserdichten Behälter",
        en: "Kept in a fireproof or watertight container",
      },
    ],
  },
  {
    id: "other",
    de: "Sonstiges",
    en: "Other",
    items: [
      {
        id: "water-container",
        de: "Wasserbehälter, für Löschwasser oder Brauchwasser",
        en: "Water containers, for firefighting or washing",
      },
      {
        id: "mask",
        de: "Atemschutzmaske, zum Schutz vor Viren, Bakterien und gefährlichen Stoffen in der Luft",
        en: "Respirator mask, against viruses, bacteria and airborne hazards",
      },
      {
        id: "helmet",
        de: "Schutzhelm, zum Schutz bei herumfliegenden Trümmern",
        en: "Hard hat, against flying debris",
      },
      { id: "cash", de: "Bargeld (zusätzlich zu Notgepäck)", en: "Cash (on top of the go bag)" },
    ],
  },
];

/** Every checklist id, for validating what came back from the server. */
export const checklistIds: Set<string> = new Set(checklist.flatMap(s => s.items.map(i => i.id)));

/**
 * Grams rendered the way the federal table prints them: litres for drinks,
 * kilograms once there is more than a kilo, grams below that.
 */
export function formatAmount(grams: number, category: EmergencyCategory): string {
  if (category === "drinks") {
    return `${round(grams / 1000)} l`;
  }
  if (grams >= 1000) {
    return `${round(grams / 1000)} kg`;
  }
  return `${Math.round(grams)} g`;
}

function round(value: number): string {
  return (Math.round(value * 10) / 10).toLocaleString();
}
